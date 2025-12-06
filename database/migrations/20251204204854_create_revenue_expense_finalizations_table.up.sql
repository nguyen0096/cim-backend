-- Create revenue_expense_finalizations table to track historical finalized dates
CREATE TABLE IF NOT EXISTS revenue_expense_finalizations (
    id SERIAL PRIMARY KEY,
    finalized_date TIMESTAMP WITH TIME ZONE NOT NULL,
    status VARCHAR(50),
    reason TEXT,
    created_by VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_by VARCHAR(255),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_revenue_expense_finalizations_finalized_date ON revenue_expense_finalizations(finalized_date);

