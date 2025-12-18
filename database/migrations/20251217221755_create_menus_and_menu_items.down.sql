-- Drop join tables first (due to foreign key constraints)
DROP TABLE IF EXISTS menu_item_products;
DROP TABLE IF EXISTS menu_inventories;
DROP TABLE IF EXISTS menu_menu_items;

-- Drop main tables
DROP TABLE IF EXISTS menu_items;
DROP TABLE IF EXISTS menus;
