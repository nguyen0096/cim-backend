package apptest

import (
	"errors"
	"fmt"
	"os"
	"regexp"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gorm.io/gorm"
)

// These specs execute the shipped migration files verbatim against the suite's real
// Postgres. Each runs inside a transaction that is always rolled back, so the DDL and
// the row edits never leak into the rest of the suite.
var _ = Describe("Maintenance account migrations", func() {
	const (
		restoreDir     = "../database/migrations/20260801000001_restore_tool_account_roles"
		removeDir      = "../database/migrations/20260801000002_remove_unrecognised_root_account"
		backupTable    = "users_role_backup_20260801000000"
		unrecognisedID = "demoRootAdminUid200000000000"
	)

	errRollback := errors.New("rollback")

	sqlOf := func(path string) string {
		raw, err := os.ReadFile(path)
		Expect(err).NotTo(HaveOccurred())
		return string(raw)
	}

	// inTx runs body in a transaction that is unconditionally rolled back.
	inTx := func(body func(tx *gorm.DB)) {
		err := tenv.DB.Transaction(func(tx *gorm.DB) error {
			body(tx)
			return errRollback
		})
		Expect(err).To(MatchError(errRollback))
	}

	insertUser := func(tx *gorm.DB, uid, role string) string {
		id := uuid.NewString()
		email := fmt.Sprintf("mig-%s@cim.local", id)
		Expect(tx.Exec(
			`INSERT INTO users (id, uid, email, name, role, type, status) VALUES (?, ?, ?, ?, ?, 'user', 'active')`,
			id, uid, email, email, role,
		).Error).NotTo(HaveOccurred())
		return id
	}

	roleOf := func(tx *gorm.DB, id string) string {
		var role string
		Expect(tx.Raw(`SELECT role FROM users WHERE id = ?`, id).Scan(&role).Error).NotTo(HaveOccurred())
		return role
	}

	Describe("20260801000001 restore_tool_account_roles", func() {
		It("returns each backed-up account to its exact prior role, and reverses", func() {
			inTx(func(tx *gorm.DB) {
				admin := insertUser(tx, uuid.NewString(), "developer")
				accountant := insertUser(tx, uuid.NewString(), "developer")
				for id, prior := range map[string]string{admin: "admin", accountant: "accountant"} {
					Expect(tx.Exec(
						fmt.Sprintf(`INSERT INTO %s (user_id, uid, previous_role) SELECT id, uid, ? FROM users WHERE id = ?`, backupTable),
						prior, id,
					).Error).NotTo(HaveOccurred())
				}

				Expect(tx.Exec(sqlOf(restoreDir + ".up.sql")).Error).NotTo(HaveOccurred())
				Expect(roleOf(tx, admin)).To(Equal("admin"))
				Expect(roleOf(tx, accountant)).To(Equal("accountant"))

				Expect(tx.Exec(sqlOf(restoreDir + ".down.sql")).Error).NotTo(HaveOccurred())
				Expect(roleOf(tx, admin)).To(Equal("developer"))
				Expect(roleOf(tx, accountant)).To(Equal("developer"))
			})
		})

		It("no-ops when the account has no backup row", func() {
			inTx(func(tx *gorm.DB) {
				id := insertUser(tx, uuid.NewString(), "developer")

				Expect(tx.Exec(sqlOf(restoreDir + ".up.sql")).Error).NotTo(HaveOccurred())
				Expect(roleOf(tx, id)).To(Equal("developer"))
			})
		})

		It("no-ops when the backup table is absent", func() {
			inTx(func(tx *gorm.DB) {
				id := insertUser(tx, uuid.NewString(), "developer")
				Expect(tx.Exec(`DROP TABLE ` + backupTable).Error).NotTo(HaveOccurred())

				Expect(tx.Exec(sqlOf(restoreDir + ".up.sql")).Error).NotTo(HaveOccurred())
				Expect(tx.Exec(sqlOf(restoreDir + ".down.sql")).Error).NotTo(HaveOccurred())
				Expect(roleOf(tx, id)).To(Equal("developer"))
			})
		})
	})

	Describe("20260801000002 remove_unrecognised_root_account", func() {
		It("soft-deletes the account when it exists, and reverses", func() {
			inTx(func(tx *gorm.DB) {
				id := insertUser(tx, unrecognisedID, "admin")

				Expect(tx.Exec(sqlOf(removeDir + ".up.sql")).Error).NotTo(HaveOccurred())
				var deleted int64
				Expect(tx.Raw(`SELECT count(*) FROM users WHERE id = ? AND deleted_at IS NOT NULL`, id).
					Scan(&deleted).Error).NotTo(HaveOccurred())
				Expect(deleted).To(Equal(int64(1)))

				Expect(tx.Exec(sqlOf(removeDir + ".down.sql")).Error).NotTo(HaveOccurred())
				Expect(tx.Raw(`SELECT count(*) FROM users WHERE id = ? AND deleted_at IS NULL`, id).
					Scan(&deleted).Error).NotTo(HaveOccurred())
				Expect(deleted).To(Equal(int64(1)))
			})
		})

		It("leaves a row that was already soft-deleted before the up ran", func() {
			inTx(func(tx *gorm.DB) {
				id := insertUser(tx, unrecognisedID, "admin")
				Expect(tx.Exec(`UPDATE users SET deleted_at = NOW() WHERE id = ?`, id).Error).NotTo(HaveOccurred())

				Expect(tx.Exec(sqlOf(removeDir + ".up.sql")).Error).NotTo(HaveOccurred())
				Expect(tx.Exec(sqlOf(removeDir + ".down.sql")).Error).NotTo(HaveOccurred())

				var live int64
				Expect(tx.Raw(`SELECT count(*) FROM users WHERE id = ? AND deleted_at IS NULL`, id).
					Scan(&live).Error).NotTo(HaveOccurred())
				Expect(live).To(Equal(int64(0)), "the down must not resurrect a row the up never deleted")
			})
		})

		It("no-ops on re-run once no live row carries the UID", func() {
			inTx(func(tx *gorm.DB) {
				var live int64
				Expect(tx.Raw(`SELECT count(*) FROM users WHERE uid = ? AND deleted_at IS NULL`, unrecognisedID).
					Scan(&live).Error).NotTo(HaveOccurred())
				Expect(live).To(Equal(int64(0)))

				res := tx.Exec(sqlOf(removeDir + ".up.sql"))
				Expect(res.Error).NotTo(HaveOccurred())
				Expect(res.RowsAffected).To(Equal(int64(0)))
			})
		})
	})

	Describe("20260801000003 drop_restaurant_admin_role", func() {
		const guardDir = "../database/migrations/20260801000003_drop_restaurant_admin_role"
		// The role literal only exists in the migration; read it back rather than
		// re-declaring it here, so the repo stays free of other references to it.
		removedRole := func() string {
			m := regexp.MustCompile(`role = '([a-z-]+)'`).FindStringSubmatch(sqlOf(guardDir + ".up.sql"))
			Expect(m).To(HaveLen(2))
			return m[1]
		}

		It("no-ops when nobody holds the removed role, which is the expected outcome", func() {
			inTx(func(tx *gorm.DB) {
				var holders int64
				Expect(tx.Raw(`SELECT count(*) FROM users WHERE role = ? AND deleted_at IS NULL`, removedRole()).
					Scan(&holders).Error).NotTo(HaveOccurred())
				Expect(holders).To(Equal(int64(0)))

				Expect(tx.Exec(sqlOf(guardDir + ".up.sql")).Error).NotTo(HaveOccurred())
				Expect(tx.Exec(sqlOf(guardDir + ".down.sql")).Error).NotTo(HaveOccurred())
			})
		})

		It("stops the migration rather than locking a holder out of everything", func() {
			inTx(func(tx *gorm.DB) {
				insertUser(tx, uuid.NewString(), removedRole())

				err := tx.Exec(sqlOf(guardDir + ".up.sql")).Error
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("still held by 1 user"))
			})
		})
	})

	// End state of the suite's own migration run, which applied every file in order.
	Describe("after the full migration run", func() {
		It("leaves the seeded maintenance account on its pre-20260801000000 role and the unrecognised one soft-deleted", func() {
			var role string
			Expect(tenv.DB.Raw(`SELECT role FROM users WHERE uid = ? AND deleted_at IS NULL`,
				"demoRootAdminUid000000000000").Scan(&role).Error).NotTo(HaveOccurred())
			Expect(role).To(Equal("admin"), "assign + restore must net to no role change")

			var deletedAt *string
			Expect(tenv.DB.Raw(`SELECT deleted_at FROM users WHERE uid = ?`,
				unrecognisedID).Scan(&deletedAt).Error).NotTo(HaveOccurred())
			Expect(deletedAt).NotTo(BeNil())
		})
	})
})
