-- +goose Up
-- Historical Wavelet→OpenFlare table rename bridge (formerly Go migration).
-- Fresh installs and current of_* schemas need no action.
SELECT 1;

-- +goose Down
SELECT 1;
