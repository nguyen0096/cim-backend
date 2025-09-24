1. Scope of Work

Login & basic authorization: Email/Password, 2 roles (Admin and Staff). (Firebase Auth)

Product & inventory management: CRUD, import/export, update stock quantities.

Order management: Create orders, update status, confirm completion → automatically deduct from inventory.

Excel update: Export data to a single Excel sheet (~10 columns) while keeping other data unchanged. (Go Excel library)

Deployment & handover: BE with Go + DB Postgres/MySQL, source code handover.

Warranty: 06 months of bug fixing within the agreed scope.

2. Models

Product:
- id
- name
- supplier_id
- status

Inventory:
- id
- product_id
- quantity

Supplier:
- id
- name

PurchaseOrder
- metadata (id, created_at, etc)
- status

Purchase Order Item
- metadata
- purchase_order_id
- product_id
- quantity