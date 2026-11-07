import type { RuntimeTranslator } from '@/i18n/runtime';
import { type PolicySection } from './types';

export const TERMS_LAST_UPDATED = '2026-06-07';

/**
 * ------------------------------------------------------------------
 * 服务条款 (TERMS OF SERVICE)
 * ------------------------------------------------------------------
 */
export function createTermsSections(rt: RuntimeTranslator): PolicySection[] {
  return [
    {
      value: 'contract-establishment',
      title: rt('1_agreement_and_service_overview_511c6782'),
      content: (
        <div className='space-y-4 text-sm leading-relaxed'>
          <p>
            <strong>{rt('1_1_parties_d2bf79cd')}</strong>{' '}
            {rt('these_terms_of_service_the_agreement_govern_1e6754fc')}
          </p>
          <p>
            <strong>{rt('1_2_read_carefully_dfde346e')}</strong>{' '}
            {rt('this_system_is_a_general_purpose_full_f1d395fe')}
          </p>
          <p>
            <strong>{rt('1_3_agreement_contents_f05cebc0')}</strong>{' '}
            {rt('the_agreement_includes_these_terms_and_any_43e244b6')}
          </p>
        </div>
      ),
    },
    {
      value: 'service-definition',
      title: rt('2_service_definition_and_nature_61db87f8'),
      content: (
        <div className='space-y-4 text-sm leading-relaxed'>
          <ul className='list-disc pl-4 md:pl-5 space-y-2'>
            <li>
              <strong>{rt('your_responsibility_527cbdbf')}</strong>{' '}
              {rt('the_entity_deploying_and_operating_a_customized_047c6a8f')}
            </li>
          </ul>
        </div>
      ),
    },
    {
      value: 'account-specifications',
      title: rt('3_registration_and_account_use_cfab1555'),
      content: (
        <div className='space-y-4 text-sm leading-relaxed'>
          <p>
            <strong>{rt('3_1_accounts_2631c114')}</strong>{' '}
            {rt('users_may_register_through_the_frontend_or_e527ef52')}
          </p>
          <p>
            <strong>{rt('3_2_password_and_token_security_80dd80d9')}</strong>
          </p>
          <ul className='list-disc pl-4 md:pl-5 space-y-2'>
            <li>
              <strong>{rt('password_security_7a29e44e')}</strong>{' '}
              {rt('keep_your_account_password_secure_eb9adf10')}
            </li>
            <li>
              <strong>{rt('api_token_security_34af066f')}</strong>{' '}
              {rt('a_personal_access_token_api_key_represents_6215b3b0')}{' '}
              <strong>
                {rt('you_are_responsible_for_data_loss_or_0c9acc88')}
              </strong>
            </li>
          </ul>
        </div>
      ),
    },
    {
      value: 'user-conduct',
      title: rt('4_prohibited_user_conduct_bd5628b6'),
      content: (
        <div className='space-y-4 text-sm leading-relaxed'>
          <p>
            {rt('when_using_this_system_you_must_comply_1e54356c')}{' '}
            <strong>
              {rt('the_following_activities_are_strictly_prohibited_5b57de7f')}
            </strong>
          </p>
          <div className='bg-red-500/5 border border-red-500/20 rounded-lg p-3 md:p-4 space-y-3'>
            <ul className='list-disc pl-4 md:pl-5 space-y-2 text-red-600 dark:text-red-400 font-medium'>
              <li>
                <strong>{rt('endangering_national_security_4a160c4e')}</strong>{' '}
                {rt(
                  'opposing_constitutional_principles_endangering_national_security_disclosing_bdfb8814',
                )}
              </li>
              <li>
                <strong>{rt('illegal_information_services_c97748bb')}</strong>{' '}
                {rt('hacking_tools_ddos_attack_services_server_brute_686746c4')}
              </li>
              <li>
                <strong>{rt('pornography_gambling_and_drugs_fdb44a54')}</strong>{' '}
                {rt(
                  'creating_copying_publishing_or_distributing_obscene_pornographic_ec8c0fdb',
                )}
              </li>
              <li>
                <strong>
                  {rt('intellectual_property_infringement_cc10b950')}
                </strong>{' '}
                {rt('selling_pirated_software_or_media_illegal_game_3f0f4159')}
              </li>
              <li>
                <strong>{rt('other_unlawful_information_4085c17c')}</strong>{' '}
                {rt(
                  'spreading_rumors_promoting_cults_or_superstition_insulting_a062981a',
                )}
              </li>
            </ul>
          </div>
          <p>
            <strong>{rt('enforcement_9c7cfd68')}</strong>{' '}
            {rt('if_these_rules_are_violated_the_platform_86df3b18')}
            <strong>
              {rt('permanently_suspend_your_account_block_all_access_b9f04a42')}
            </strong>
          </p>
        </div>
      ),
    },
    {
      value: 'liability-limitation',
      title: rt('5_disclaimer_and_force_majeure_be649f43'),
      content: (
        <div className='space-y-4 text-sm leading-relaxed'>
          <p>
            <strong>{rt('5_1_general_disclaimer_30d01f0d')}</strong>{' '}
            {rt('this_development_platform_is_provided_as_is_23bc2b55')}
          </p>
          <p>
            <strong>{rt('5_2_service_interruptions_16558fa8')}</strong>{' '}
            {rt('the_platform_is_not_liable_for_interruptions_405b1c95')}
          </p>
          <ul className='list-disc pl-4 md:pl-5 space-y-1 text-muted-foreground'>
            <li>
              {rt(
                'natural_disasters_including_typhoons_earthquakes_tsunamis_and_1bbbfb21',
              )}
            </li>
            <li>
              {rt(
                'government_actions_cybersecurity_law_changes_or_administrative_2b7322f5',
              )}
            </li>
            <li>
              {rt('telecommunication_faults_or_damage_to_data_center_259a542f')}
            </li>
            <li>
              {rt('data_corruption_or_downtime_caused_by_hacking_68f2e04f')}
            </li>
          </ul>
        </div>
      ),
    },
    {
      value: 'governing-law',
      title: rt('6_governing_law_and_disputes_b9663c86'),
      content: (
        <div className='space-y-4 text-sm leading-relaxed'>
          <p>
            <strong>{rt('6_1_governing_law_ff2afc5e')}</strong>{' '}
            {rt(
              'the_formation_performance_interpretation_and_resolution_of_c1510775',
            )}
            <strong>{rt('the_laws_of_the_people_s_republic_5d5c80d6')}</strong>{' '}
            {rt('excluding_the_conflict_of_laws_rules_of_b235c4e3')}
          </p>
          <p>
            <strong>{rt('6_2_dispute_resolution_44e871de')}</strong>{' '}
            {rt('disputes_should_first_be_resolved_through_friendly_1ccf4610')}
          </p>
        </div>
      ),
    },
  ];
}
