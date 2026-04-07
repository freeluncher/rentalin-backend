CREATE TABLE IF NOT EXISTS product_items (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    product_id TEXT NOT NULL,
    serial_number TEXT NOT NULL,
    condition_status TEXT NOT NULL DEFAULT 'good',
    availability_status TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT product_items_availability_status_chk
        CHECK (availability_status IN ('available', 'reserved', 'rented', 'maintenance', 'lost')),
    CONSTRAINT product_items_tenant_serial_unq UNIQUE (tenant_id, serial_number)
);

CREATE INDEX IF NOT EXISTS idx_product_items_tenant_availability
    ON product_items (tenant_id, availability_status);

CREATE TABLE IF NOT EXISTS rentals (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    customer_name TEXT NOT NULL,
    start_at TIMESTAMPTZ NOT NULL,
    due_at TIMESTAMPTZ NOT NULL,
    returned_at TIMESTAMPTZ NULL,
    status TEXT NOT NULL,
    subtotal NUMERIC(14,2) NOT NULL DEFAULT 0,
    total_fees NUMERIC(14,2) NOT NULL DEFAULT 0,
    grand_total NUMERIC(14,2) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT rentals_status_chk
        CHECK (status IN ('draft', 'reserved', 'active', 'partially_returned', 'completed', 'cancelled')),
    CONSTRAINT rentals_due_after_start_chk CHECK (due_at > start_at)
);

CREATE INDEX IF NOT EXISTS idx_rentals_tenant_status
    ON rentals (tenant_id, status);

CREATE INDEX IF NOT EXISTS idx_rentals_tenant_due_at
    ON rentals (tenant_id, due_at);

CREATE TABLE IF NOT EXISTS rental_items (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    rental_id TEXT NOT NULL,
    product_item_id TEXT NOT NULL,
    daily_rate NUMERIC(14,2) NOT NULL,
    planned_days INT NOT NULL,
    actual_days INT NOT NULL DEFAULT 0,
    line_total NUMERIC(14,2) NOT NULL,
    status TEXT NOT NULL,
    returned_at TIMESTAMPTZ NULL,
    return_condition TEXT NULL,
    damage_assessment NUMERIC(14,2) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT rental_items_status_chk
        CHECK (status IN ('reserved', 'rented', 'returned', 'lost')),
    CONSTRAINT rental_items_planned_days_chk CHECK (planned_days >= 1),
    CONSTRAINT rental_items_actual_days_chk CHECK (actual_days >= 0),
    CONSTRAINT rental_items_daily_rate_chk CHECK (daily_rate >= 0),
    CONSTRAINT rental_items_damage_assessment_chk CHECK (damage_assessment >= 0),
    CONSTRAINT rental_items_rental_fk FOREIGN KEY (rental_id) REFERENCES rentals (id) ON DELETE CASCADE,
    CONSTRAINT rental_items_product_item_fk FOREIGN KEY (product_item_id) REFERENCES product_items (id)
);

CREATE INDEX IF NOT EXISTS idx_rental_items_tenant_rental
    ON rental_items (tenant_id, rental_id);

CREATE INDEX IF NOT EXISTS idx_rental_items_tenant_product
    ON rental_items (tenant_id, product_item_id);

CREATE TABLE IF NOT EXISTS fee_lines (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    rental_id TEXT NOT NULL,
    rental_item_id TEXT NULL,
    fee_type TEXT NOT NULL,
    amount NUMERIC(14,2) NOT NULL,
    notes TEXT NOT NULL DEFAULT '',
    created_by TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fee_lines_fee_type_chk CHECK (fee_type IN ('late', 'damage', 'other')),
    CONSTRAINT fee_lines_amount_chk CHECK (amount >= 0),
    CONSTRAINT fee_lines_rental_fk FOREIGN KEY (rental_id) REFERENCES rentals (id) ON DELETE CASCADE,
    CONSTRAINT fee_lines_rental_item_fk FOREIGN KEY (rental_item_id) REFERENCES rental_items (id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_fee_lines_tenant_rental
    ON fee_lines (tenant_id, rental_id);

CREATE INDEX IF NOT EXISTS idx_fee_lines_tenant_type
    ON fee_lines (tenant_id, fee_type);
