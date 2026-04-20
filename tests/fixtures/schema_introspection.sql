DROP TABLE IF EXISTS sample_measurements;
DROP TABLE IF EXISTS audit_events;
DROP TABLE IF EXISTS order_items;
DROP TABLE IF EXISTS orders;
DROP TABLE IF EXISTS addresses;
DROP TABLE IF EXISTS products;
DROP TABLE IF EXISTS customers;

CREATE TABLE customers (
    id INT AUTO_INCREMENT PRIMARY KEY COMMENT 'Customer identifier',
    name VARCHAR(255) NOT NULL COMMENT 'Customer display name',
    customer_code VARCHAR(64) NOT NULL COMMENT 'External customer code',
    profile JSON NULL COMMENT 'Customer profile document',
    notes TEXT NULL COMMENT 'Free-form notes',
    created_at DATETIME NOT NULL COMMENT 'Customer creation timestamp',
    UNIQUE KEY customers_customer_code_key (customer_code)
) COMMENT='Customer master data';

CREATE TABLE addresses (
    id INT AUTO_INCREMENT PRIMARY KEY COMMENT 'Address identifier',
    customer_id INT NOT NULL COMMENT 'Owning customer',
    city VARCHAR(128) NOT NULL COMMENT 'City name',
    country_code VARCHAR(2) NOT NULL COMMENT 'ISO country code',
    created_at DATETIME NOT NULL COMMENT 'Address creation timestamp',
    CONSTRAINT fk_addresses_customers
        FOREIGN KEY (customer_id) REFERENCES customers (id)
) COMMENT='Shipping and billing addresses';

CREATE TABLE products (
    id INT AUTO_INCREMENT PRIMARY KEY COMMENT 'Product identifier',
    sku VARCHAR(64) NOT NULL COMMENT 'Stock keeping unit',
    name VARCHAR(255) NOT NULL COMMENT 'Product name',
    price DECIMAL(10,2) NOT NULL COMMENT 'Current unit price',
    metadata JSON NULL COMMENT 'Product metadata payload',
    created_at DATETIME NOT NULL COMMENT 'Product creation timestamp',
    UNIQUE KEY products_sku_key (sku)
) COMMENT='Catalog products';

CREATE TABLE orders (
    id INT AUTO_INCREMENT PRIMARY KEY COMMENT 'Order identifier',
    customer_id INT NOT NULL COMMENT 'Customer placing the order',
    shipping_address_id INT NOT NULL COMMENT 'Shipping address reference',
    status VARCHAR(32) NOT NULL DEFAULT 'pending' COMMENT 'Current order status',
    total_amount DECIMAL(12,2) NOT NULL COMMENT 'Order total amount',
    placed_at DATETIME NOT NULL COMMENT 'Order placement timestamp',
    CONSTRAINT fk_orders_customers
        FOREIGN KEY (customer_id) REFERENCES customers (id),
    CONSTRAINT fk_orders_addresses
        FOREIGN KEY (shipping_address_id) REFERENCES addresses (id)
) COMMENT='Customer orders';

CREATE TABLE order_items (
    order_id INT NOT NULL COMMENT 'Order reference',
    line_number INT NOT NULL COMMENT 'Line ordinal',
    product_id INT NOT NULL COMMENT 'Product reference',
    quantity INT NOT NULL COMMENT 'Ordered quantity',
    unit_price DECIMAL(10,2) NOT NULL COMMENT 'Unit price at order time',
    PRIMARY KEY (order_id, line_number),
    CONSTRAINT fk_order_items_orders
        FOREIGN KEY (order_id) REFERENCES orders (id),
    CONSTRAINT fk_order_items_products
        FOREIGN KEY (product_id) REFERENCES products (id)
) COMMENT='Order line items';

CREATE TABLE audit_events (
    id INT AUTO_INCREMENT PRIMARY KEY COMMENT 'Audit event identifier',
    order_id INT NOT NULL COMMENT 'Order reference',
    payload JSON NULL COMMENT 'Event payload',
    details TEXT NULL COMMENT 'Human-readable event details',
    happened_at DATETIME NOT NULL COMMENT 'Event timestamp',
    CONSTRAINT fk_audit_events_orders
        FOREIGN KEY (order_id) REFERENCES orders (id)
) COMMENT='Order audit events';
