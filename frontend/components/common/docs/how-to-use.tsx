import type { RuntimeTranslator } from '@/i18n/runtime';
import { type PolicySection } from './types';
import { CodeBlock } from '@/components/ui/code-block';

/**
 * ------------------------------------------------------------------
 * 使用指南 (How To Use)
 * ------------------------------------------------------------------
 */
export function createHowToUseSections(rt: RuntimeTranslator): PolicySection[] {
  return [
    {
      value: 'quick-start',
      title: rt('1_quick_start_ef8fbb58'),
      content: (
        <div className='space-y-4 text-sm leading-relaxed'>
          <div className='bg-muted/50 border border-border/50 rounded-lg px-3 py-2 mb-6'>
            <p className='text-muted-foreground m-0'>
              {rt('a_guide_to_the_general_purpose_full_9cd8c987')}
            </p>
          </div>
          <ul className='list-disc pl-5 space-y-2'>
            <li>
              <strong>{rt('architecture_e992b65c')}</strong>{' '}
              {rt('go_gin_gorm_redis_asynq_backend_react_d5fe02d3')}
            </li>
            <li>
              <strong>{rt('authentication_d36b345b')}</strong>{' '}
              {rt(
                'local_username_password_registration_and_login_plus_583ab638',
              )}
            </li>
            <li>
              <strong>{rt('access_tokens_b7b856d3')}</strong>{' '}
              {rt('personal_developer_access_tokens_api_keys_for_4b407149')}
            </li>
            <li>
              <strong>{rt('observability_ea0cc1bf')}</strong>{' '}
              {rt(
                'zap_structured_logging_and_opentelemetry_distributed_tracing_9fd6da9f',
              )}
            </li>
          </ul>
        </div>
      ),
    },
    {
      value: 'auth-security',
      title: rt('2_authentication_and_security_settings_76967059'),
      content: (
        <div className='space-y-4 text-sm leading-relaxed'>
          <p>
            {rt(
              'the_platform_combines_authentication_methods_to_support_bc9f1772',
            )}
          </p>
          <h3
            id='2-1-login'
            className='text-base md:text-lg font-semibold text-foreground mt-4 mb-2'
          >
            {rt('2_1_username_and_password_authentication_97184f8f')}
          </h3>
          <ul className='list-disc pl-4 md:pl-5 space-y-2'>
            <li>
              <strong>{rt('registration_and_password_login_c5bd6cb4')}</strong>{' '}
              {rt('users_can_register_and_create_sessions_with_21883e04')}{' '}
              <code className='bg-muted px-1.5 py-0.5 rounded text-xs font-mono'>
                bcrypt
              </code>{' '}
              {rt('salted_hashing_dc2a9b63')}
            </li>
            <li>
              <strong>{rt('system_switches_53741557')}</strong>{' '}
              {rt(
                'administrators_can_disable_password_registration_or_login_a2648e62',
              )}
            </li>
          </ul>

          <h3
            id='2-2-oidc'
            className='text-base md:text-lg font-semibold text-foreground mt-4 mb-2'
          >
            {rt('2_2_third_party_oidc_providers_f14083c8')}
          </h3>
          <p>{rt('users_can_link_external_accounts_from_their_c08d3f5a')}</p>
          <ol className='list-decimal pl-4 md:pl-5 space-y-1'>
            <li>
              {rt('open_4540d74d')}
              <strong>{rt('settings_profile_32835b0d')}</strong>{' '}
              {rt('message_a173cce8')}
            </li>
            <li>
              {rt(
                'under_linked_accounts_view_existing_connections_or_5151eb9b',
              )}
            </li>
            <li>{rt('after_linking_use_the_oidc_login_button_6095b15d')}</li>
          </ol>
        </div>
      ),
      children: [
        {
          value: '2-1-login',
          title: rt('2_1_username_and_password_authentication_97184f8f'),
        },
        {
          value: '2-2-oidc',
          title: rt('2_2_third_party_oidc_providers_f14083c8'),
        },
      ],
    },
    {
      value: 'access-token',
      title: rt('3_integrating_personal_access_tokens_af2647b5'),
      content: (
        <div className='space-y-4 text-sm leading-relaxed'>
          <p>
            {rt('personal_access_tokens_allow_developers_and_third_9a242a0e')}
          </p>
          <h3
            id='3-1-generation'
            className='text-base md:text-lg font-semibold text-foreground mt-4 mb-2'
          >
            {rt('3_1_token_generation_and_storage_2b78c681')}
          </h3>
          <ul className='list-disc pl-4 md:pl-5 space-y-2'>
            <li>
              <strong>{rt('one_time_plaintext_display_fd13912a')}</strong>{' '}
              {rt('a_newly_generated_plaintext_token_such_as_57eb11a1')}{' '}
              <code className='bg-muted px-1.5 py-0.5 rounded text-xs font-mono'>
                at_xxx
              </code>
              {rt('is_displayed_only_once_copy_and_save_8d378c8a')}
            </li>
            <li>
              <strong>{rt('secure_hash_storage_1ce392ff')}</strong>{' '}
              {rt('only_a_sha_256_hash_of_the_dbf434b1')}
            </li>
          </ul>

          <h3
            id='3-2-usage'
            className='text-base md:text-lg font-semibold text-foreground mt-4 mb-2'
          >
            {rt('3_2_calling_apis_with_headers_fccd7b0d')}
          </h3>
          <p>{rt('use_your_saved_plaintext_token_with_either_3af53a05')}</p>
          <div className='space-y-2'>
            <p className='font-semibold text-xs text-muted-foreground'>
              {rt('method_1_authorization_bearer_header_2a9c2a6c')}
            </p>
            <CodeBlock
              code={`GET /api/v1/user/self HTTP/1.1
Host: localhost:3000
Authorization: Bearer at_628d022b7a95e26b...`}
              language='http'
            />
          </div>
          <div className='space-y-2'>
            <p className='font-semibold text-xs text-muted-foreground'>
              {rt('method_2_custom_x_access_token_header_4548169f')}
            </p>
            <CodeBlock
              code={`GET /api/v1/user/self HTTP/1.1
Host: localhost:3000
X-Access-Token: at_628d022b7a95e26b...`}
              language='http'
            />
          </div>
        </div>
      ),
      children: [
        {
          value: '3-1-generation',
          title: rt('3_1_token_generation_and_storage_2b78c681'),
        },
        {
          value: '3-2-usage',
          title: rt('3_2_calling_apis_with_headers_fccd7b0d'),
        },
      ],
    },
    {
      value: 'config-system',
      title: rt('4_dynamic_system_configuration_b3bf8c49'),
      content: (
        <div className='space-y-4 text-sm leading-relaxed'>
          <p>{rt('the_built_in_key_value_configuration_module_713f6d05')}</p>
          <ul className='list-disc pl-4 md:pl-5 space-y-2'>
            <li>
              <strong>{rt('cached_reads_35828322')}</strong>{' '}
              {rt('configuration_is_read_from_the_database_through_3e9f6a34')}
            </li>
            <li>
              <strong>{rt('core_system_configuration_keys_6dc02be8')}</strong>
              <ul className='list-[circle] pl-5 mt-1 space-y-1 text-muted-foreground'>
                <li>
                  <code className='bg-muted px-1 rounded text-xs font-mono'>
                    site_name
                  </code>
                  {rt('platform_display_name_116219b1')}
                </li>
                <li>
                  <code className='bg-muted px-1 rounded text-xs font-mono'>
                    password_login_enabled
                  </code>
                  {rt('password_login_switch_153eaa45')}
                </li>
                <li>
                  <code className='bg-muted px-1 rounded text-xs font-mono'>
                    registration_enabled
                  </code>
                  {rt('user_registration_switch_827b5b21')}
                </li>
                <li>
                  <code className='bg-muted px-1 rounded text-xs font-mono'>
                    max_api_keys_per_user
                  </code>
                  {rt('maximum_tokens_per_regular_user_default_5_5dd6d9e7')}
                </li>
              </ul>
            </li>
          </ul>
        </div>
      ),
    },
    {
      value: 'worker-scheduler',
      title: rt('5_background_tasks_and_scheduling_7142ce96'),
      content: (
        <div className='space-y-4 text-sm leading-relaxed'>
          <p>
            {rt('cobra_provides_separate_commands_so_worker_services_1bbae2df')}
          </p>
          <ul className='list-disc pl-4 md:pl-5 space-y-2'>
            <li>
              <strong>
                {rt('scheduled_dispatch_scheduler_command_e79062ad')}
              </strong>{' '}
              {rt('enqueues_tasks_in_redis_according_to_their_694e5985')}
            </li>
            <li>
              <strong>
                {rt('workers_with_queue_priorities_worker_command_faa970b1')}
              </strong>{' '}
              {rt('asynq_consumes_tasks_by_priority_and_executes_69d82251')}
            </li>
          </ul>
        </div>
      ),
    },
    {
      value: 'tracing-metrics',
      title: rt('6_distributed_tracing_and_structured_logs_e8971cad'),
      content: (
        <div className='space-y-4 text-sm leading-relaxed'>
          <p>
            {rt(
              'the_platform_integrates_monitoring_components_for_observability_2b1da50f',
            )}
          </p>
          <ul className='list-disc pl-4 md:pl-5 space-y-2'>
            <li>
              <strong>{rt('ui_openTelemetryTracing')}</strong>{' '}
              {rt(
                'tracing_context_is_propagated_automatically_gin_requests_0f549cce',
              )}
            </li>
            <li>
              <strong>{rt('zap_structured_logs_1c9725e8')}</strong>{' '}
              {rt('standardizes_backend_console_and_log_output_as_16970757')}
            </li>
          </ul>
        </div>
      ),
    },
  ];
}
