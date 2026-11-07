import type { RuntimeTranslator } from '@/i18n/runtime';
import { type PolicySection } from './types';
import { CodeBlock } from '@/components/ui/code-block';
import {
  DocsTable,
  DocsTableBody,
  DocsTableCell,
  DocsTableHead,
  DocsTableHeader,
  DocsTableRow,
} from '@/components/ui/docs-table';

export const DOCS_LAST_UPDATED = '2026-06-07';

/**
 * ------------------------------------------------------------------
 * API 文档
 * ------------------------------------------------------------------
 */
export function createApiSections(rt: RuntimeTranslator): PolicySection[] {
  return [
    {
      value: 'api-specs',
      title: rt('1_api_conventions_and_authentication_e6d4df61'),
      content: (
        <div className='space-y-4 text-sm leading-relaxed'>
          <div className='bg-muted/50 border border-border/50 rounded-lg px-3 py-2 mb-6'>
            <p className='text-muted-foreground m-0'>
              {rt(
                'standard_api_request_conventions_and_developer_access_39eb956e',
              )}
            </p>
          </div>

          <h3
            id='1-1-response-format'
            className='text-base md:text-lg font-semibold text-foreground mt-6 mb-3'
          >
            {rt('1_1_standard_response_format_5cddae2f')}
          </h3>
          <p>{rt('all_api_endpoints_use_a_standard_json_87c92fb0')}</p>
          <DocsTable>
            <DocsTableHeader>
              <DocsTableRow>
                <DocsTableHead className='w-[120px]'>
                  {rt('field_49ecd0e3')}
                </DocsTableHead>
                <DocsTableHead className='w-[100px]'>
                  {rt('type_ba40014f')}
                </DocsTableHead>
                <DocsTableHead>{rt('description_4262c45d')}</DocsTableHead>
              </DocsTableRow>
            </DocsTableHeader>
            <DocsTableBody>
              <DocsTableRow>
                <DocsTableCell className='font-mono text-xs'>
                  error_msg
                </DocsTableCell>
                <DocsTableCell>string</DocsTableCell>
                <DocsTableCell>
                  {rt('error_message_empty_on_success_contains_error_d89d457f')}
                </DocsTableCell>
              </DocsTableRow>
              <DocsTableRow>
                <DocsTableCell className='font-mono text-xs'>
                  data
                </DocsTableCell>
                <DocsTableCell>any</DocsTableCell>
                <DocsTableCell>
                  {rt('the_response_payload_null_when_the_request_97b8b29c')}
                </DocsTableCell>
              </DocsTableRow>
            </DocsTableBody>
          </DocsTable>

          <p className='mt-2'>{rt('successful_response_example_8917e853')}</p>
          <CodeBlock
            code={`{
  "error_msg": "",
  "data": {
    "id": 1,
    "username": "ryan",
    "nickname": "Ryan"
  }
}`}
            language='json'
          />

          <p className='mt-2'>{rt('failed_response_example_eae899e1')}</p>
          <CodeBlock
            code={rt.raw('error_msg_incorrect_password_data_null_b2eccc86')}
            language='json'
          />

          <h3
            id='1-2-authentication'
            className='text-base md:text-lg font-semibold text-foreground mt-6 mb-3'
          >
            {rt('1_2_authentication_methods_3c5db38a')}
          </h3>
          <p>
            {rt(
              'protected_endpoints_require_credentials_except_public_endpoints_1fa0a24c',
            )}
          </p>
          <ul className='list-disc pl-5 space-y-2'>
            <li>
              <strong>{rt('session_credentials_de72024d')}</strong>{' '}
              {rt('browser_clients_can_stay_signed_in_using_f316279b')}
            </li>
            <li>
              <strong>{rt('access_tokens_8de23550')}</strong>{' '}
              {rt('for_backend_calls_and_third_party_integrations_dfd8bf0f')}
            </li>
          </ul>
          <div className='bg-muted border rounded-xl p-4 mt-2 space-y-2'>
            <p className='font-bold text-xs'>
              {rt('use_either_of_the_following_request_headers_1e03b04b')}
            </p>
            <ul className='list-disc pl-5 text-xs text-muted-foreground space-y-1'>
              <li>
                <code className='bg-muted-foreground/10 px-1 rounded text-[11px] font-mono'>
                  Authorization: Bearer at_xxx
                </code>
              </li>
              <li>
                <code className='bg-muted-foreground/10 px-1 rounded text-[11px] font-mono'>
                  X-Access-Token: at_xxx
                </code>
              </li>
            </ul>
          </div>
        </div>
      ),
      children: [
        {
          value: '1-1-response-format',
          title: rt('1_1_standard_response_format_5cddae2f'),
        },
        {
          value: '1-2-authentication',
          title: rt('1_2_authentication_methods_3c5db38a'),
        },
      ],
    },
    {
      value: 'auth-apis',
      title: rt('2_user_and_authentication_endpoints_4a89d429'),
      content: (
        <div className='space-y-4 text-sm leading-relaxed'>
          <h3
            id='2-1-register'
            className='text-base md:text-lg font-semibold text-foreground mt-4 mb-2'
          >
            {rt('2_1_user_registration_4f332f85')}
          </h3>
          <p>
            <strong>{rt('endpoint_79a2369a')}</strong> POST{' '}
            <code className='bg-muted px-1.5 py-0.5 rounded text-xs font-mono'>
              /api/v1/user/register
            </code>
          </p>
          <p>
            <strong>{rt('description_3e633a20')}</strong>{' '}
            {rt('register_a_local_account_when_registration_is_938891dc')}
          </p>
          <DocsTable>
            <DocsTableHeader>
              <DocsTableRow>
                <DocsTableHead className='w-[120px]'>
                  {rt('parameter_9634fb08')}
                </DocsTableHead>
                <DocsTableHead className='w-[80px]'>
                  {rt('required_11da9dc4')}
                </DocsTableHead>
                <DocsTableHead>{rt('type_ba40014f')}</DocsTableHead>
                <DocsTableHead>{rt('description_4262c45d')}</DocsTableHead>
              </DocsTableRow>
            </DocsTableHeader>
            <DocsTableBody>
              <DocsTableRow>
                <DocsTableCell className='font-mono text-xs'>
                  username
                </DocsTableCell>
                <DocsTableCell>{rt('yes_b5141d3d')}</DocsTableCell>
                <DocsTableCell>string</DocsTableCell>
                <DocsTableCell>
                  {rt('username_must_be_unique_and_contain_no_e81c1e5c')}
                </DocsTableCell>
              </DocsTableRow>
              <DocsTableRow>
                <DocsTableCell className='font-mono text-xs'>
                  password
                </DocsTableCell>
                <DocsTableCell>{rt('yes_b5141d3d')}</DocsTableCell>
                <DocsTableCell>string</DocsTableCell>
                <DocsTableCell>
                  {rt('password_at_least_8_characters_37cdc807')}
                </DocsTableCell>
              </DocsTableRow>
              <DocsTableRow>
                <DocsTableCell className='font-mono text-xs'>
                  nickname
                </DocsTableCell>
                <DocsTableCell>{rt('no_0c70665b')}</DocsTableCell>
                <DocsTableCell>string</DocsTableCell>
                <DocsTableCell>
                  {rt(
                    'nickname_defaults_to_the_username_when_omitted_676aae7e',
                  )}
                </DocsTableCell>
              </DocsTableRow>
            </DocsTableBody>
          </DocsTable>

          <h3
            id='2-2-login'
            className='text-base md:text-lg font-semibold text-foreground mt-6 mb-2'
          >
            {rt('2_2_password_login_271483ba')}
          </h3>
          <p>
            <strong>{rt('endpoint_79a2369a')}</strong> POST{' '}
            <code className='bg-muted px-1.5 py-0.5 rounded text-xs font-mono'>
              /api/v1/user/login
            </code>
          </p>
          <p>
            <strong>{rt('description_3e633a20')}</strong>{' '}
            {rt(
              'authenticate_with_a_username_and_password_establishing_a38ff252',
            )}
          </p>
          <DocsTable>
            <DocsTableHeader>
              <DocsTableRow>
                <DocsTableHead className='w-[120px]'>
                  {rt('parameter_9634fb08')}
                </DocsTableHead>
                <DocsTableHead className='w-[80px]'>
                  {rt('required_11da9dc4')}
                </DocsTableHead>
                <DocsTableHead>{rt('type_ba40014f')}</DocsTableHead>
                <DocsTableHead>{rt('description_4262c45d')}</DocsTableHead>
              </DocsTableRow>
            </DocsTableHeader>
            <DocsTableBody>
              <DocsTableRow>
                <DocsTableCell className='font-mono text-xs'>
                  username
                </DocsTableCell>
                <DocsTableCell>{rt('yes_b5141d3d')}</DocsTableCell>
                <DocsTableCell>string</DocsTableCell>
                <DocsTableCell>{rt('username_1a3f0617')}</DocsTableCell>
              </DocsTableRow>
              <DocsTableRow>
                <DocsTableCell className='font-mono text-xs'>
                  password
                </DocsTableCell>
                <DocsTableCell>{rt('yes_b5141d3d')}</DocsTableCell>
                <DocsTableCell>string</DocsTableCell>
                <DocsTableCell>{rt('password_a621ab60')}</DocsTableCell>
              </DocsTableRow>
            </DocsTableBody>
          </DocsTable>

          <h3
            id='2-3-logout'
            className='text-base md:text-lg font-semibold text-foreground mt-6 mb-2'
          >
            {rt('2_3_sign_out_3252611d')}
          </h3>
          <p>
            <strong>{rt('endpoint_79a2369a')}</strong> GET{' '}
            <code className='bg-muted px-1.5 py-0.5 rounded text-xs font-mono'>
              /api/v1/user/logout
            </code>
          </p>
          <p>
            <strong>{rt('description_3e633a20')}</strong>{' '}
            {rt('delete_the_current_session_cookie_and_sign_c213ffd9')}
          </p>

          <h3
            id='2-4-profile'
            className='text-base md:text-lg font-semibold text-foreground mt-6 mb-2'
          >
            {rt('2_4_get_profile_7b96110a')}
          </h3>
          <p>
            <strong>{rt('endpoint_79a2369a')}</strong> GET{' '}
            <code className='bg-muted px-1.5 py-0.5 rounded text-xs font-mono'>
              /api/v1/user/self
            </code>
          </p>
          <p>
            <strong>{rt('description_3e633a20')}</strong>{' '}
            {rt('get_the_signed_in_account_s_basic_b0eb8cfa')}
          </p>
        </div>
      ),
      children: [
        { value: '2-1-register', title: rt('2_1_user_registration_4f332f85') },
        { value: '2-2-login', title: rt('2_2_password_login_271483ba') },
        { value: '2-3-logout', title: rt('2_3_sign_out_3252611d') },
        { value: '2-4-profile', title: rt('2_4_get_profile_7b96110a') },
      ],
    },
    {
      value: 'token-apis',
      title: rt('3_personal_access_token_endpoints_989f5d7c'),
      content: (
        <div className='space-y-4 text-sm leading-relaxed'>
          <div className='bg-muted/50 border border-border/50 rounded-lg px-3 py-2 mb-4'>
            <p className='text-muted-foreground m-0'>
              {rt('access_token_management_requires_a_session_login_42d91543')}
            </p>
          </div>

          <h3
            id='3-1-list-token'
            className='text-base md:text-lg font-semibold text-foreground mt-4 mb-2'
          >
            {rt('3_1_list_tokens_9840c227')}
          </h3>
          <p>
            <strong>{rt('endpoint_79a2369a')}</strong> GET{' '}
            <code className='bg-muted px-1.5 py-0.5 rounded text-xs font-mono'>
              /api/v1/user/access-tokens
            </code>
          </p>
          <p>
            <strong>{rt('description_3e633a20')}</strong>{' '}
            {rt('list_the_current_user_s_tokens_with_c34d2210')}
          </p>

          <h3
            id='3-2-create-token'
            className='text-base md:text-lg font-semibold text-foreground mt-6 mb-2'
          >
            {rt('3_2_create_an_access_token_e53bd784')}
          </h3>
          <p>
            <strong>{rt('endpoint_79a2369a')}</strong> POST{' '}
            <code className='bg-muted px-1.5 py-0.5 rounded text-xs font-mono'>
              /api/v1/user/access-tokens
            </code>
          </p>
          <p>
            <strong>{rt('parameters_ba89685c')}</strong> {rt('ui_jsonBody')}{' '}
            <code className='bg-muted px-1.5 rounded text-xs font-mono'>
              {rt.raw('name_token_name_is_admin_false_4f3e4868')}
            </code>
          </p>
          <p>
            <strong>{rt('description_3e633a20')}</strong>{' '}
            {rt('generate_an_access_token_the_response_includes_70552390')}
          </p>
          <p className='mt-1 text-xs text-muted-foreground'>
            <code className='bg-muted px-1 rounded'>is_admin</code>
            {rt('optional_default_210b9d8f')}{' '}
            <code className='bg-muted px-1 rounded'>false</code>
            {rt('grant_administrator_privileges_to_the_token_only_2292cad8')}{' '}
            <code className='bg-muted px-1 rounded'>/admin/**</code>{' '}
            {rt('endpoints_815c88e5')}
          </p>
          <p className='mt-2'>{rt('successful_response_example_7783a39c')}</p>
          <CodeBlock
            code={`{
  "error_msg": "",
  "data": {
    "token": "at_628d022b7a95e26bcd8b29c9...",
    "record": {
      "id": 5,
      "user_id": 1,
      "name": "my-dev-key",
      "masked_token": "at_628d...29c9",
      "is_admin": false,
      "created_at": "2026-06-07T21:30:00+08:00"
    }
  }
}`}
            language='json'
          />

          <h3
            id='3-3-delete-token'
            className='text-base md:text-lg font-semibold text-foreground mt-6 mb-2'
          >
            {rt('3_3_revoke_delete_a_token_ae510f58')}
          </h3>
          <p>
            <strong>{rt('endpoint_79a2369a')}</strong> DELETE{' '}
            <code className='bg-muted px-1.5 py-0.5 rounded text-xs font-mono'>
              /api/v1/user/access-tokens/:id
            </code>
          </p>
          <p>
            <strong>{rt('description_3e633a20')}</strong>{' '}
            {rt('permanently_delete_a_token_by_id_immediately_145287f6')}
          </p>

          <h3
            id='3-4-rotate-token'
            className='text-base md:text-lg font-semibold text-foreground mt-6 mb-2'
          >
            {rt('3_4_rotate_a_token_secret_79209983')}
          </h3>
          <p>
            <strong>{rt('endpoint_79a2369a')}</strong> POST{' '}
            <code className='bg-muted px-1.5 py-0.5 rounded text-xs font-mono'>
              /api/v1/user/access-tokens/:id/rotate
            </code>
          </p>
          <p>
            <strong>{rt('description_3e633a20')}</strong>{' '}
            {rt('replace_a_token_s_secret_and_return_d2caca89')}
          </p>
        </div>
      ),
      children: [
        { value: '3-1-list-token', title: rt('3_1_list_tokens_9840c227') },
        {
          value: '3-2-create-token',
          title: rt('3_2_create_an_access_token_e53bd784'),
        },
        {
          value: '3-3-delete-token',
          title: rt('3_3_revoke_delete_a_token_ae510f58'),
        },
        {
          value: '3-4-rotate-token',
          title: rt('3_4_rotate_a_token_secret_79209983'),
        },
      ],
    },
    {
      value: 'config-apis',
      title: rt('4_public_configuration_and_administration_17ebdbd5'),
      content: (
        <div className='space-y-4 text-sm leading-relaxed'>
          <h3
            id='4-1-public-config'
            className='text-base md:text-lg font-semibold text-foreground mt-4 mb-2'
          >
            {rt('4_1_public_system_configuration_2be912e4')}
          </h3>
          <p>
            <strong>{rt('endpoint_79a2369a')}</strong> GET{' '}
            <code className='bg-muted px-1.5 py-0.5 rounded text-xs font-mono'>
              /api/v1/config/public
            </code>
          </p>
          <p>
            <strong>{rt('description_3e633a20')}</strong>{' '}
            {rt('get_publicly_visible_configuration_keys_and_values_bc7bff66')}
          </p>
          <p className='mt-2'>{rt('response_example_d3087445')}</p>
          <CodeBlock
            code={`{
  "error_msg": "",
  "data": {
    "site_name": "OpenFlare",
    "registration_enabled": "false",
    "password_login_enabled": "true",
    "password_register_enabled": "false",
    "cap_login_enabled": "false",
    "oidc_login_enabled": "true"
  }
}`}
            language='json'
          />

          <h3
            id='4-2-admin-configs'
            className='text-base md:text-lg font-semibold text-foreground mt-6 mb-2'
          >
            {rt('4_2_system_configuration_crud_administrator_2ce66c8b')}
          </h3>
          <p>
            <strong>{rt('description_3e633a20')}</strong>{' '}
            {rt(
              'manage_system_configs_dynamically_through_an_administrator_acdcdb99',
            )}
          </p>
          <ul className='list-disc pl-5 space-y-2'>
            <li>
              <strong>{rt('list_configuration_3d58680b')}</strong> GET{' '}
              <code className='bg-muted px-1 rounded text-xs font-mono'>
                /api/v1/admin/system-configs?type=system
              </code>
            </li>
            <li>
              <strong>{rt('create_a_configuration_entry_1d995264')}</strong>{' '}
              POST{' '}
              <code className='bg-muted px-1 rounded text-xs font-mono'>
                /api/v1/admin/system-configs
              </code>
            </li>
            <li>
              <strong>{rt('update_a_configuration_value_2a0b0b76')}</strong> PUT{' '}
              <code className='bg-muted px-1 rounded text-xs font-mono'>
                /api/v1/admin/system-configs/:key
              </code>
            </li>
            <li>
              <strong>{rt('delete_a_configuration_entry_1b77a6ea')}</strong>{' '}
              DELETE{' '}
              <code className='bg-muted px-1 rounded text-xs font-mono'>
                /api/v1/admin/system-configs/:key
              </code>
            </li>
          </ul>
        </div>
      ),
      children: [
        {
          value: '4-1-public-config',
          title: rt('4_1_public_system_configuration_2be912e4'),
        },
        {
          value: '4-2-admin-configs',
          title: rt('4_2_system_configuration_crud_administrator_2ce66c8b'),
        },
      ],
    },
  ];
}
