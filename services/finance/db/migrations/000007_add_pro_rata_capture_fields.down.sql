ALTER TABLE finance.pro_rata_schedules
DROP CONSTRAINT pro_rata_schedules_status_check;

ALTER TABLE finance.pro_rata_schedules
ADD CONSTRAINT pro_rata_schedules_status_check
CHECK (status IN ('pending', 'applied'));

ALTER TABLE finance.pro_rata_schedules
DROP COLUMN failure_reason;

ALTER TABLE finance.pro_rata_schedules
DROP COLUMN captured_rate_snapshot;

ALTER TABLE finance.pro_rata_schedules
DROP COLUMN creation_reporting_currency;

ALTER TABLE finance.pro_rata_schedules
DROP COLUMN transaction_currency;

ALTER TABLE finance.pro_rata_schedules
DROP COLUMN transaction_amount;
