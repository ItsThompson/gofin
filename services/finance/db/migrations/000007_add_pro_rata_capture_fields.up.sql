ALTER TABLE finance.pro_rata_schedules
ADD COLUMN transaction_amount BIGINT;

ALTER TABLE finance.pro_rata_schedules
ADD COLUMN transaction_currency VARCHAR(3);

ALTER TABLE finance.pro_rata_schedules
ADD COLUMN creation_reporting_currency VARCHAR(3);

ALTER TABLE finance.pro_rata_schedules
ADD COLUMN captured_rate_snapshot JSONB;

ALTER TABLE finance.pro_rata_schedules
ADD COLUMN failure_reason VARCHAR(50);

ALTER TABLE finance.pro_rata_schedules
DROP CONSTRAINT pro_rata_schedules_status_check;

ALTER TABLE finance.pro_rata_schedules
ADD CONSTRAINT pro_rata_schedules_status_check
CHECK (status IN ('pending', 'applied', 'failed'));
