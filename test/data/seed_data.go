package data

import (
	"cim-backend/database"
	"cim-backend/internal/auth"
	"cim-backend/internal/config"
	"cim-backend/internal/models"
	"cim-backend/internal/repository"
	"cim-backend/internal/services"
	"cim-backend/pkg"
	"context"
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"
)

// Suppliers contains all test supplier data
func Suppliers() []models.Supplier {
	now := time.Now()

	return []models.Supplier{
		// F&B Suppliers (Vietnam)
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:         "Công ty Nông Sản Sạch Việt Nam",
			ContactEmail: "contact@nongsansach.vn",
			ContactPhone: "+84-28-3821-5001",
			Address:      "123 Đường Nguyễn Văn Linh, Quận 7, TP.HCM",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:         "Vinamilk - Công ty Sữa Việt Nam",
			ContactEmail: "sales@vinamilk.com.vn",
			ContactPhone: "+84-28-5413-8888",
			Address:      "10 Đường Tân Trào, Phường Tân Phú, Quận 7, TP.HCM",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:         "Công ty Thủy Sản Miền Trung",
			ContactEmail: "info@thuysanmientrung.vn",
			ContactPhone: "+84-236-3827-100",
			Address:      "45 Đường Nguyễn Văn Linh, TP. Đà Nẵng",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:         "Trung Nguyên Coffee",
			ContactEmail: "order@trungnguyen.com.vn",
			ContactPhone: "+84-500-6789",
			Address:      "12 Đường Thảo Điền, Quận 2, TP.HCM",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:         "Công ty Gạo Thiên Long",
			ContactEmail: "sales@gaothienlong.vn",
			ContactPhone: "+84-292-3821-456",
			Address:      "234 Quốc Lộ 1A, TP. Cần Thơ",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:         "Công ty Gia Vị Việt",
			ContactEmail: "contact@giaviviet.com",
			ContactPhone: "+84-28-3920-7777",
			Address:      "678 Đường Lê Văn Việt, Quận 9, TP.HCM",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:         "Vissan - Công ty Thực Phẩm Sài Gòn",
			ContactEmail: "orders@vissan.com.vn",
			ContactPhone: "+84-28-3812-5555",
			Address:      "520 Đường Cách Mạng Tháng Tám, Quận 3, TP.HCM",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:         "Công ty Rau Quả Đà Lạt",
			ContactEmail: "sales@rauquadalat.vn",
			ContactPhone: "+84-263-3821-999",
			Address:      "89 Đường Trần Hưng Đạo, TP. Đà Lạt",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:         "Công ty Nước Mắm Nam Ngư",
			ContactEmail: "info@namngu.com.vn",
			ContactPhone: "+84-297-3871-234",
			Address:      "101 Đường Trần Phú, TP. Phú Quốc",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:         "Công ty Bánh Kẹo Kinh Đô",
			ContactEmail: "contact@kinhdo.com.vn",
			ContactPhone: "+84-28-5413-7000",
			Address:      "443 Đường Hoàng Văn Thụ, Quận Tân Bình, TP.HCM",
		},
	}
}

type ProductSeed struct {
	Base        models.Base
	Name        string
	Description string
	ProductType string
	UnitSymbol  string
	Status      string
}

// Units returns the measurement units used in test data
func Units() []models.Unit {
	now := time.Now()

	return []models.Unit{
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			UnitType:         "mass",
			Name:             "KILOGRAM",
			Symbol:           "kg",
			ConversionFactor: 1,
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			UnitType:         "volume",
			Name:             "LITER",
			Symbol:           "liter",
			ConversionFactor: 1,
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			UnitType:         "count",
			Name:             "PIECE",
			Symbol:           "piece",
			ConversionFactor: 1,
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			UnitType:         "count",
			Name:             "BOX",
			Symbol:           "box",
			ConversionFactor: 1,
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			UnitType:         "count",
			Name:             "CARTON",
			Symbol:           "carton",
			ConversionFactor: 1,
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			UnitType:         "count",
			Name:             "BOTTLE",
			Symbol:           "bottle",
			ConversionFactor: 1,
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			UnitType:         "count",
			Name:             "CAN",
			Symbol:           "can",
			ConversionFactor: 1,
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			UnitType:         "count",
			Name:             "PACK",
			Symbol:           "pack",
			ConversionFactor: 1,
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			UnitType:         "count",
			Name:             "LOAF",
			Symbol:           "loaf",
			ConversionFactor: 1,
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			UnitType:         "count",
			Name:             "TRAY",
			Symbol:           "tray",
			ConversionFactor: 1,
		},
	}
}

// productSeeds contains all test product data
func productSeeds() []ProductSeed {
	now := time.Now()

	return []ProductSeed{
		// F&B Products (Vietnam)
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Gạo Tám Thơm ST25",
			Description: "Gạo thơm ST25 đặc sản xuất khẩu, loại 1",
			ProductType: "Cơm",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Cà Phê Robusta Đắk Lắk",
			Description: "Hạt cà phê Robusta nguyên chất từ Tây Nguyên",
			ProductType: "Nước",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Nước Mắm Phú Quốc 40 Độ Đạm",
			Description: "Nước mắm truyền thống Phú Quốc, 40 độ đạm đạm protein",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "liter",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Bánh Mì Tươi Sài Gòn",
			Description: "Bánh mì que giòn tươi ngon mỗi ngày",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "piece",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Rau Xà Lách Đà Lạt",
			Description: "Rau xà lách tươi hữu cơ từ Đà Lạt",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Sữa Tươi Vinamilk 100%",
			Description: "Sữa tươi thanh trùng không đường",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "liter",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Tôm Càng Xanh Cần Thơ",
			Description: "Tôm càng xanh tươi sống từ đồng bằng sông Cửu Long",
			ProductType: "Cơm",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Chả Lụa Đặc Biệt",
			Description: "Chả lụa thượng hạng chất lượng cao",
			ProductType: "Cơm",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Trà Ô Long Đài Loan",
			Description: "Trà ô long cao cấp nhập khẩu từ Đài Loan",
			ProductType: "Nước",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Bánh Quy Bơ Kinh Đô",
			Description: "Bánh quy bơ thơm ngon đặc biệt",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "box",
			Status:      "active",
		},
		// Additional F&B Products
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Bún Tươi",
			Description: "Bún tươi dai ngon từ gạo tẻ",
			ProductType: "Cơm",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Phở Khô",
			Description: "Bánh phở khô loại 1",
			ProductType: "Cơm",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Mì Gói Hảo Hảo",
			Description: "Mì ăn liền vị tôm chua cay",
			ProductType: "Cơm",
			UnitSymbol:  "carton",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Dầu Ăn Simply",
			Description: "Dầu ăn cao cấp chai 1L",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "liter",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Nước Tương Chinsu",
			Description: "Nước tương đậm đà hương vị truyền thống",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "liter",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Tương Ớt Cholimex",
			Description: "Tương ớt cay đặc biệt",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "bottle",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Đường Trắng Biên Hòa",
			Description: "Đường cát trắng tinh luyện",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Muối I-ốt",
			Description: "Muối tinh I-ốt sạch",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Bột Ngọt Aji-ngon",
			Description: "Bột ngọt tăng vị tự nhiên",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Hạt Nêm Knorr",
			Description: "Hạt nêm thịt thăn xương ống heo",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Cá Ngừ Đóng Hộp VisanFoods",
			Description: "Cá ngừ xốt cà chua 170g",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "can",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Sữa Đặc Ông Thọ",
			Description: "Sữa đặc có đường truyền thống",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "can",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Sữa Chua Vinamilk",
			Description: "Sữa chua có đường lốc 4 hộp",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "pack",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Trứng Gà Tươi",
			Description: "Trứng gà sạch các loại",
			ProductType: "Cơm",
			UnitSymbol:  "tray",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Thịt Ba Chỉ Heo",
			Description: "Thịt ba chỉ heo tươi VietGAP",
			ProductType: "Cơm",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Thịt Nạc Vai Heo",
			Description: "Thịt nạc vai heo tươi",
			ProductType: "Cơm",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Thịt Gà Ta",
			Description: "Thịt gà ta sạch",
			ProductType: "Cơm",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Cá Basa Phi Lê",
			Description: "Cá basa phi lê đông lạnh",
			ProductType: "Cơm",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Cá Thu Tươi",
			Description: "Cá thu biển tươi ngon",
			ProductType: "Cơm",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Mực Ống Đông Lạnh",
			Description: "Mực ống sạch đông lạnh",
			ProductType: "Cơm",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Rau Muống",
			Description: "Rau muống tươi",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Cải Thảo",
			Description: "Cải thảo Đà Lạt",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Cà Chua",
			Description: "Cà chua chín đỏ",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Hành Tây",
			Description: "Hành tây tím Đà Lạt",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Tỏi",
			Description: "Tỏi tươi Lý Sơn",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Ớt",
			Description: "Ớt hiểm các loại",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Khoai Tây",
			Description: "Khoai tây Đà Lạt",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Su Su",
			Description: "Su su tươi Đà Lạt",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Cà Rốt",
			Description: "Cà rốt Đà Lạt",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Bí Đỏ",
			Description: "Bí đỏ ngọt tự nhiên",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Chuối Già",
			Description: "Chuối già chín tự nhiên",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Cam Sành",
			Description: "Cam sành Hà Giang",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Xoài Cát Hòa Lộc",
			Description: "Xoài cát Hòa Lộc Tiền Giang",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Thanh Long Ruột Đỏ",
			Description: "Thanh long ruột đỏ Bình Thuận",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Sầu Riêng Monthong",
			Description: "Sầu riêng Monthong chuẩn xuất khẩu",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Măng Cụt",
			Description: "Măng cụt tươi miền Tây",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Chôm Chôm",
			Description: "Chôm chôm tươi ngọt",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Dưa Hấu Không Hạt",
			Description: "Dưa hấu không hạt ngọt lịm",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Bơ Booth",
			Description: "Bơ Booth Đắk Lắk",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Dừa Xiêm",
			Description: "Dừa xiêm tươi mát",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "piece",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Bia Sài Gòn Xanh",
			Description: "Bia Sài Gòn xanh lager chai 330ml",
			ProductType: "Nước",
			UnitSymbol:  "bottle",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Bia Tiger",
			Description: "Bia Tiger lon 330ml",
			ProductType: "Nước",
			UnitSymbol:  "can",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Nước Suối Lavie",
			Description: "Nước khoáng thiên nhiên Lavie 500ml",
			ProductType: "Nước",
			UnitSymbol:  "bottle",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Nước Ngọt Coca Cola",
			Description: "Coca Cola lon 330ml",
			ProductType: "Nước",
			UnitSymbol:  "can",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Nước Ngọt Pepsi",
			Description: "Pepsi Cola lon 330ml",
			ProductType: "Nước",
			UnitSymbol:  "can",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Trà Xanh Không Độ",
			Description: "Trà xanh không độ C2 chai 455ml",
			ProductType: "Nước",
			UnitSymbol:  "bottle",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Nước Cam Ép Minute Maid",
			Description: "Nước cam ép 100% chai 1L",
			ProductType: "Nước",
			UnitSymbol:  "bottle",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Sữa Tươi TH True Milk",
			Description: "Sữa tươi tiệt trùng hộp 1L",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "box",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Yaourt Uống Dutch Lady",
			Description: "Yaourt uống lốc 4 chai",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "pack",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Bánh Mì Sandwich",
			Description: "Bánh mì sandwich cắt lát",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "loaf",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Bánh Bông Lan Trứng Muối",
			Description: "Bánh bông lan trứng muối thơm ngon",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "piece",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Bánh Mì Que Việt Nam",
			Description: "Bánh mì que giòn tan",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "piece",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Bánh Croissant Bơ",
			Description: "Bánh croissant bơ thơm ngậy",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "piece",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Bánh Pía Tân Huê Viên",
			Description: "Bánh pía đậu xanh sầu riêng",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "box",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Bánh Trung Thu Kinh Đô",
			Description: "Bánh trung thu thập cẩm cao cấp",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "box",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Nem Chua Thanh Hóa",
			Description: "Nem chua thanh hóa truyền thống",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Kẹo Dừa Bến Tre",
			Description: "Kẹo dừa thơm ngậy đặc sản",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "box",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Mứt Tết Hỗn Hợp",
			Description: "Mứt tết gừng, bí, dừa, cà rốt",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "box",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Snack Oishi Vị Tôm",
			Description: "Snack khoai tây Oishi vị tôm",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "pack",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Hạt Điều Rang Muối",
			Description: "Hạt điều rang muối Bình Phước",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Mực Tẩm Gia Vị",
			Description: "Mực tẩm gia vị cay ngọt",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "pack",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Xúc Xích Đức Việt",
			Description: "Xúc xích heo đặc biệt",
			ProductType: "Cơm",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Giò Lụa",
			Description: "Giò lụa thượng hạng",
			ProductType: "Cơm",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Giò Thủ",
			Description: "Giò thủ truyền thống",
			ProductType: "Cơm",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Chả Quế",
			Description: "Chả quế thơm ngon",
			ProductType: "Cơm",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Pate Gan Heo",
			Description: "Pate gan heo cao cấp",
			ProductType: "Cơm",
			UnitSymbol:  "can",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Mật Ong Rừng U Minh",
			Description: "Mật ong nguyên chất rừng U Minh",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "bottle",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Bột Canh Heo Quay",
			Description: "Bột canh heo quay đậm đà",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "pack",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Hạt Tiêu Phú Quốc",
			Description: "Hạt tiêu đen nguyên hạt Phú Quốc",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Bột Nghệ Nguyên Chất",
			Description: "Bột nghệ nguyên chất không tẩm",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Sả Tươi",
			Description: "Sả tươi thơm mạnh",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Gừng Tươi",
			Description: "Gừng tươi cay nồng",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Nước Dừa Đóng Hộp",
			Description: "Nước dừa tươi đóng hộp Cocoxim",
			ProductType: "Nước",
			UnitSymbol:  "can",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Nước Mía Đóng Chai",
			Description: "Nước mía tươi đóng chai",
			ProductType: "Nước",
			UnitSymbol:  "bottle",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Trà Atiso Đà Lạt",
			Description: "Trà atiso giải nhiệt thanh lọc",
			ProductType: "Nước",
			UnitSymbol:  "bottle",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Trà Sữa Lipton",
			Description: "Trá sữa lon 250ml",
			ProductType: "Nước",
			UnitSymbol:  "can",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Mì Chính Ajinomoto",
			Description: "Mì chính tinh khiết 400g",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "pack",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Giấm Gạo Nhật Bản",
			Description: "Giấm gạo nấu ăn Nhật Bản",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "bottle",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Dầu Hào Lee Kum Kee",
			Description: "Dầu hào đặc biệt Lee Kum Kee",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "bottle",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Bột Chiên Giòn",
			Description: "Bột chiên xù giòn lâu",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Bột Mì Đa Dụng",
			Description: "Bột mì đa dụng số 8",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Bột Năng",
			Description: "Bột năng tinh khiết",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Bột Gạo",
			Description: "Bột gạo làm bánh",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Bột Nở",
			Description: "Bột nở làm bánh",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "pack",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Men Nở Bánh",
			Description: "Men nở bánh mì instant",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "pack",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Bơ Thực Vật",
			Description: "Bơ thực vật làm bánh",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Cream Cheese Philadelphia",
			Description: "Phô mai kem làm bánh",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Phô Mai Lát Cheddar",
			Description: "Phô mai lát Cheddar hộp 200g",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "box",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Dừa Nạo Khô",
			Description: "Dừa nạo sợi khô",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Đậu Phộng Rang",
			Description: "Đậu phộng rang giòn",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Mè Rang",
			Description: "Mè rang trắng thơm",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Đậu Xanh Hạt",
			Description: "Đậu xanh hạt nguyên vỏ",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Đậu Đen",
			Description: "Đậu đen hạt nguyên chất",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Đậu Đỏ",
			Description: "Đậu đỏ hạt làm chè",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Nấm Hương Khô",
			Description: "Nấm hương khô cao cấp",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Miến Dong",
			Description: "Miến dong miền Bắc",
			ProductType: "Cơm",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Hủ Tiếu Khô Nam Vang",
			Description: "Hủ tiếu khô Nam Vang đặc biệt",
			ProductType: "Cơm",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Bánh Đa Nem",
			Description: "Bánh đa nem cuốn loại 1",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "pack",
			Status:      "active",
		},
	}
}

// Products converts product seeds into persisted models with resolved unit IDs
func Products(unitIDs map[string]uint) []models.Product {
	seeds := productSeeds()
	products := make([]models.Product, 0, len(seeds))

	for _, seed := range seeds {
		unitID, ok := unitIDs[seed.UnitSymbol]
		if !ok {
			if fallback, hasFallback := unitIDs["piece"]; hasFallback {
				unitID = fallback
			} else {
				continue
			}
		}

		products = append(products, models.Product{
			Base:        seed.Base,
			Name:        seed.Name,
			Description: seed.Description,
			ProductType: seed.ProductType,
			UnitID:      unitID,
			Status:      seed.Status,
		})
	}

	return products
}

// InventoryData represents inventory configuration for a product
type InventoryData struct {
	ProductID    uint
	SupplierID   uint
	UnitPrice    float64
	UnitType     string
	Quantity     float64
	ReorderLevel int
	Location     string
}

// Inventory contains all test inventory data
func Inventory(productIDs []uint) []models.Inventory {
	now := time.Now()

	// Create inventory locations
	inventories := []models.Inventory{
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Main Warehouse A",
			Description: "Primary storage facility for electronics and office supplies",
			Location:    "123 Industrial Blvd, San Francisco, CA 94107",
			Status:      models.InventoryStatusActive,
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Secondary Warehouse B",
			Description: "Secondary storage facility for bulk items",
			Location:    "456 Storage Way, Oakland, CA 94607",
			Status:      models.InventoryStatusActive,
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Distribution Center C",
			Description: "Distribution center for fast-moving items",
			Location:    "789 Logistics Ave, San Jose, CA 95110",
			Status:      models.InventoryStatusActive,
		},
	}

	return inventories
}

// createProductSupplierRelationships creates many-to-many relationships between products and suppliers
func createProductSupplierRelationships(tx *gorm.DB, productIDs, supplierIDs []uint) error {
	// F&B products supplied by multiple F&B suppliers (each product gets 3-5 suppliers)
	type ProductSupplier struct {
		ProductID  uint `gorm:"primaryKey"`
		SupplierID uint `gorm:"primaryKey"`
	}
	var productSuppliers []ProductSupplier

	for i, productID := range productIDs {
		// F&B products: assign to multiple F&B suppliers (each product gets 3-5 suppliers)
		// Calculate how many suppliers this product should have (3-5)
		numSuppliers := 3 + (i % 3)         // Will give 3, 4, or 5 suppliers per product
		fbSupplierCount := len(supplierIDs) // All suppliers are now F&B suppliers

		for j := 0; j < numSuppliers; j++ {
			// Distribute suppliers across products with some variation
			supplierOffset := (i*2 + j) % fbSupplierCount
			supplierIndex := supplierOffset
			productSuppliers = append(productSuppliers, ProductSupplier{
				ProductID:  productID,
				SupplierID: supplierIDs[supplierIndex],
			})
		}
	}

	if err := tx.Table("product_suppliers").Create(&productSuppliers).Error; err != nil {
		return fmt.Errorf("failed to create product-supplier relationships: %w", err)
	}

	return nil
}

// SeedDatabase populates the database with mock data
func SeedDatabase() error {
	// Load configuration
	cfg := config.Load()

	// Initialize database
	db, err := database.Initialize(cfg.Database)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}

	// Create context with user email for CreatedBy field
	ctx := pkg.WithUserEmail(context.Background(), "seeder@test.com")

	// Start transaction
	tx := db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to start transaction: %w", tx.Error)
	}

	// Seed in correct order (respecting foreign key constraints)

	// 0. Base Units (Level 1)
	baseUnits := Units()
	if err := tx.Create(&baseUnits).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to create base units: %w", err)
	}

	// 0.1. Derived Units (Levels 2-4)
	derivedUnits, err := createDerivedUnits(tx, baseUnits)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to create derived units: %w", err)
	}

	// 0.2. Build unitIDs map for product creation
	allUnits := append(baseUnits, derivedUnits...)
	unitIDs := make(map[string]uint, len(allUnits))
	for _, unit := range allUnits {
		unitIDs[unit.Symbol] = unit.ID
	}

	// 1. Suppliers
	suppliers := Suppliers()
	if err := tx.Create(&suppliers).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to create suppliers: %w", err)
	}
	// Collect supplier IDs after batch creation
	var supplierIDs []uint
	for _, supplier := range suppliers {
		supplierIDs = append(supplierIDs, supplier.ID)
	}

	// 2. Products
	products := Products(unitIDs)
	if err := tx.Create(&products).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to create products: %w", err)
	}
	// Collect product IDs after batch creation
	var productIDs []uint
	for _, product := range products {
		productIDs = append(productIDs, product.ID)
	}

	// 2.5. Create product-supplier relationships
	if err := createProductSupplierRelationships(tx, productIDs, supplierIDs); err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to create product-supplier relationships: %w", err)
	}

	// 3. Inventories
	inventories := Inventory(productIDs)
	if err := tx.Create(&inventories).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to create inventories: %w", err)
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Seed users after database schema is populated
	if err := seedUsers(db, cfg.Casbin); err != nil {
		return fmt.Errorf("failed to seed users: %w", err)
	}

	return nil
}

// seedUsers populates the database with default users
func seedUsers(db *gorm.DB, casbinCfg config.CasbinConfig) error {
	// Initialize Casbin service
	casbinService, err := auth.NewCasbinService(db, casbinCfg)
	if err != nil {
		return fmt.Errorf("failed to initialize Casbin service: %w", err)
	}

	// Initialize user repository and service
	userRepo := repository.NewUserRepository(db, "development")
	userService := services.NewUserService(userRepo, casbinService)

	ctx := context.Background()

	// Define default users
	defaultUsers := []struct {
		UID   string
		Email string
		Name  string
		Role  string
	}{
		{
			UID:   "demoAdminUid0000000000000000",
			Email: "test@cim.local",
			Name:  "Admin User",
			Role:  string(models.RoleAdmin),
		},
		{
			UID:   "demoRootAdminUid000000000000",
			Email: "admin@example.com",
			Name:  "Admin User",
			Role:  string(models.RoleAdmin),
		},
		{
			UID:   "demoRootAdminUid200000000000",
			Email: "admin2@example.com",
			Name:  "Admin User",
			Role:  string(models.RoleAdmin),
		},
		{
			UID:   "demoAccountantUid00000000000",
			Email: "accountant@cim.local",
			Name:  "Accountant User",
			Role:  string(models.RoleAccountant),
		},
		{
			UID:   "demoStaffUid0000000000000000",
			Email: "staff@cim.local",
			Name:  "Staff User",
			Role:  string(models.RoleStaff),
		},
	}

	// Seed each user
	for _, userData := range defaultUsers {
		// Create user
		_, err = userService.CreateUser(ctx, userData.UID, userData.Email, userData.Name, userData.Role, "active")
		if err != nil {
			log.Printf("Failed to create user %s: %v", userData.Email, err)
			continue
		}

		log.Printf("Created user: %s with role: %s", userData.Email, userData.Role)
	}

	log.Println("User seeding completed!")
	return nil
}

// createDerivedUnits creates derived units (levels 2-4) based on the base units
func createDerivedUnits(tx *gorm.DB, baseUnits []models.Unit) ([]models.Unit, error) {
	now := time.Now()

	// Build map for easy lookup of base units by symbol
	unitMap := make(map[string]*models.Unit)
	for i := range baseUnits {
		unitMap[baseUnits[i].Symbol] = &baseUnits[i]
	}

	// Level 2 units
	level2Units := []models.Unit{
		// Mass hierarchy - Level 2: GRAM
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			UnitType:         "mass",
			Name:             "GRAM",
			Symbol:           "g",
			ConversionFactor: 0.001,
			Level:            2,
			BaseUnitID:       &unitMap["kg"].ID,
		},
		// Volume hierarchy - Level 2: MILLILITER
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			UnitType:         "volume",
			Name:             "MILLILITER",
			Symbol:           "ml",
			ConversionFactor: 0.001,
			Level:            2,
			BaseUnitID:       &unitMap["liter"].ID,
		},
		// Count hierarchies - Level 2
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			UnitType:         "count",
			Name:             "BOX_CARTON",
			Symbol:           "box_c",
			ConversionFactor: 0.25,
			Level:            2,
			BaseUnitID:       &unitMap["carton"].ID,
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			UnitType:         "count",
			Name:             "MILLILITER_BOTTLE",
			Symbol:           "ml_btl",
			ConversionFactor: 0.001,
			Level:            2,
			BaseUnitID:       &unitMap["bottle"].ID,
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			UnitType:         "count",
			Name:             "MILLILITER_CAN",
			Symbol:           "ml_can",
			ConversionFactor: 0.00303,
			Level:            2,
			BaseUnitID:       &unitMap["can"].ID,
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			UnitType:         "count",
			Name:             "PIECE_TRAY",
			Symbol:           "piece_t",
			ConversionFactor: 0.08333,
			Level:            2,
			BaseUnitID:       &unitMap["tray"].ID,
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			UnitType:         "count",
			Name:             "SLICE",
			Symbol:           "slice",
			ConversionFactor: 0.05,
			Level:            2,
			BaseUnitID:       &unitMap["loaf"].ID,
		},
	}

	if err := tx.Create(&level2Units).Error; err != nil {
		return nil, fmt.Errorf("failed to create level 2 units: %w", err)
	}

	// Add level 2 units to map
	for i := range level2Units {
		unitMap[level2Units[i].Symbol] = &level2Units[i]
	}

	// Level 3 units
	level3Units := []models.Unit{
		// Mass hierarchy - Level 3: MILLIGRAM
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			UnitType:         "mass",
			Name:             "MILLIGRAM",
			Symbol:           "mg",
			ConversionFactor: 0.001,
			Level:            3,
			BaseUnitID:       &unitMap["g"].ID,
		},
		// Volume hierarchy - Level 3: MICROLITER
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			UnitType:         "volume",
			Name:             "MICROLITER",
			Symbol:           "μl",
			ConversionFactor: 0.001,
			Level:            3,
			BaseUnitID:       &unitMap["ml"].ID,
		},
		// Count hierarchy - Level 3: PACK_CARTON
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			UnitType:         "count",
			Name:             "PACK_CARTON",
			Symbol:           "pack_c",
			ConversionFactor: 0.5,
			Level:            3,
			BaseUnitID:       &unitMap["box_c"].ID,
		},
	}

	if err := tx.Create(&level3Units).Error; err != nil {
		return nil, fmt.Errorf("failed to create level 3 units: %w", err)
	}

	// Add level 3 units to map
	for i := range level3Units {
		unitMap[level3Units[i].Symbol] = &level3Units[i]
	}

	// Level 4 units
	level4Units := []models.Unit{
		// Mass hierarchy - Level 4: MICROGRAM
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			UnitType:         "mass",
			Name:             "MICROGRAM",
			Symbol:           "mcg",
			ConversionFactor: 0.001,
			Level:            4,
			BaseUnitID:       &unitMap["mg"].ID,
		},
		// Volume hierarchy - Level 4: NANOLITER
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			UnitType:         "volume",
			Name:             "NANOLITER",
			Symbol:           "nl",
			ConversionFactor: 0.001,
			Level:            4,
			BaseUnitID:       &unitMap["μl"].ID,
		},
		// Count hierarchy - Level 4: PIECE_CARTON
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			UnitType:         "count",
			Name:             "PIECE_CARTON",
			Symbol:           "piece_c",
			ConversionFactor: 0.1,
			Level:            4,
			BaseUnitID:       &unitMap["pack_c"].ID,
		},
	}

	if err := tx.Create(&level4Units).Error; err != nil {
		return nil, fmt.Errorf("failed to create level 4 units: %w", err)
	}

	// Combine all derived units
	allDerived := append(level2Units, level3Units...)
	allDerived = append(allDerived, level4Units...)

	return allDerived, nil
}
