import type { RuntimeTranslator } from '@/i18n/runtime';
import { type PolicySection } from './types';

/**
 * ------------------------------------------------------------------
 * 隐私政策 (PRIVACY POLICY)
 * ------------------------------------------------------------------
 */
export function createPrivacySections(rt: RuntimeTranslator): PolicySection[] {
  return [
    {
      value: 'collection-details',
      title: rt('1_information_we_collect_af0db091'),
      content: (
        <div className='space-y-4 text-sm leading-relaxed'>
          <p>{rt('we_collect_only_the_information_necessary_to_72f6e528')}</p>
          <div className='space-y-3'>
            <div>
              <span className='font-semibold text-foreground'>
                {rt('1_1_authentication_information_60199ee9')}
              </span>
              <p className='mt-1 text-muted-foreground'>
                {rt('when_you_register_locally_or_sign_in_acdd6bd1')}{' '}
                <strong>
                  {rt('we_do_not_require_a_phone_number_3540dfca')}
                </strong>
              </p>
            </div>
            <div>
              <span className='font-semibold text-foreground'>
                {rt('1_2_service_and_api_logs_46c9a04d')}
              </span>
              <p className='mt-1 text-muted-foreground'>
                {rt(
                  'for_system_security_and_auditing_we_automatically_89737fdd',
                )}
              </p>
            </div>
          </div>
        </div>
      ),
    },
    {
      value: 'storage-security',
      title: rt('2_data_storage_and_security_7cc5cdef'),
      content: (
        <div className='space-y-4 text-sm leading-relaxed'>
          <p>{rt('we_recognize_the_importance_of_data_security_cd3bd4d9')}</p>
          <ul className='list-disc pl-4 md:pl-5 space-y-2'>
            <li>
              <strong>{rt('storage_security_a33cfbef')}</strong>{' '}
              {rt('user_data_is_stored_in_a_dedicated_132af7bb')}
            </li>
            <li>
              <strong>{rt('cryptography_9a15a88b')}</strong>{' '}
              {rt(
                'sensitive_password_and_personal_access_token_hashes_9f724cd8',
              )}
            </li>
            <li>
              <strong>{rt('access_control_3471eafa')}</strong>{' '}
              {rt('we_apply_the_principle_of_least_privilege_79328c28')}
            </li>
          </ul>
        </div>
      ),
    },
    {
      value: 'usage-rules',
      title: rt('3_how_we_use_information_41b4d6ad'),
      content: (
        <div className='space-y-4 text-sm leading-relaxed'>
          <p>{rt('collected_information_is_used_only_for_the_4868b538')}</p>
          <ul className='list-disc pl-4 md:pl-5 space-y-1'>
            <li>
              <strong>{rt('identification_481b07f4')}</strong>{' '}
              {rt('to_identify_your_registered_account_and_display_8c559094')}
            </li>
            <li>
              <strong>{rt('service_functions_c5834f20')}</strong>{' '}
              {rt(
                'to_identify_and_process_api_authentication_requests_d48a36a2',
              )}
            </li>
            <li>
              <strong>{rt('security_e7132437')}</strong>{' '}
              {rt('to_analyze_ip_addresses_and_activity_logs_5210836a')}
            </li>
          </ul>
          <p>
            <strong>{rt('prohibited_uses_b13fde94')}</strong>{' '}
            {rt('we_will_8856fb95')}
            <strong>{rt('never_c1d7d603')}</strong>{' '}
            {rt('sell_your_data_to_third_parties_or_7bb4d77a')}
          </p>
        </div>
      ),
    },
    {
      value: 'sharing-disclosure',
      title: rt('4_information_sharing_and_disclosure_c034d295'),
      content: (
        <div className='space-y-4 text-sm leading-relaxed'>
          <p>
            <strong>{rt('4_1_sharing_principles_b89eb890')}</strong>{' '}
            {rt('we_do_not_share_personal_information_with_ffcf1e8f')}
          </p>
          <ul className='list-disc pl-4 md:pl-5 space-y-1 text-muted-foreground'>
            <li>
              {rt('with_your_prior_explicit_authorization_or_consent_0fa05878')}
            </li>
            <li>
              {rt(
                'when_required_by_applicable_laws_legal_proceedings_cb547dd6',
              )}
            </li>
          </ul>
          <p>
            <strong>{rt('4_2_transfer_and_public_disclosure_383d35be')}</strong>{' '}
            {rt('we_do_not_transfer_your_personal_information_e3339921')}
          </p>
        </div>
      ),
    },
    {
      value: 'user-rights',
      title: rt('5_your_rights_and_data_management_08999326'),
      content: (
        <div className='space-y-4 text-sm leading-relaxed'>
          <p>
            {rt('under_the_personal_information_protection_law_of_94ea405e')}
          </p>
          <div className='space-y-3'>
            <div>
              <span className='font-semibold text-foreground'>
                {rt('5_1_access_and_management_854d0793')}
              </span>
              <p className='mt-1 text-muted-foreground'>
                {rt('you_may_sign_in_at_any_time_8727c5be')}
              </p>
            </div>
            <div>
              <span className='font-semibold text-foreground'>
                {rt('5_2_deletion_and_account_closure_9d1e7aba')}
              </span>
              <p className='mt-1 text-muted-foreground'>
                {rt('if_you_stop_using_the_service_you_24c642c6')}
              </p>
            </div>
          </div>
        </div>
      ),
    },
    {
      value: 'policy-update',
      title: rt('6_policy_updates_and_notices_444076f5'),
      content: (
        <div className='space-y-4 text-sm leading-relaxed'>
          <p>{rt('this_privacy_policy_may_change_as_the_2a270464')}</p>
          <p>
            {rt('we_will_prominently_announce_material_changes_for_210fbd3b')}
          </p>
        </div>
      ),
    },
  ];
}
