-- +goose Up
UPDATE w_schedules SET hide_success_log = TRUE
WHERE (id = 103 AND task_type = 'of_waf_ip_group_sync')
   OR (id = 104 AND task_type = 'of_uptime_kuma_sync');

-- +goose Down
UPDATE w_schedules SET hide_success_log = FALSE
WHERE (id = 103 AND task_type = 'of_waf_ip_group_sync')
   OR (id = 104 AND task_type = 'of_uptime_kuma_sync');
