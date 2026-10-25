import { createTranslator } from 'next-intl';
import english from '@/messages/fragments/runtime.en.json';
import chinese from '@/messages/fragments/runtime.zh-CN.json';
import russian from '@/messages/fragments/runtime.ru.json';
import type { AppLocale } from './config';

export type RuntimeKey = keyof typeof english.runtime;

const translators = {
  en: createTranslator({
    locale: 'en',
    messages: english,
    namespace: 'runtime',
  }),
  'zh-CN': createTranslator({
    locale: 'zh-CN',
    messages: chinese,
    namespace: 'runtime',
  }),
  ru: createTranslator({
    locale: 'ru',
    messages: russian,
    namespace: 'runtime',
  }),
};

export function getRuntimeTranslator(locale: AppLocale) {
  return translators[locale];
}
