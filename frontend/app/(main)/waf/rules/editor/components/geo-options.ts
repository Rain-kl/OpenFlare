import { allCountries } from 'country-region-data';

import { createCountryOptions, type GeoOption } from '@/lib/utils/geo-options';

export type { GeoOption } from '@/lib/utils/geo-options';

const regionCodePattern = /^[A-Z0-9]{1,3}$/;
export function createGeoOptions(locale: string) {
  const countryDisplayNames = new Intl.DisplayNames([locale], {
    type: 'region',
  });

  function localizedCountryName(code: string, fallback: string) {
    const localized = countryDisplayNames.of(code);
    return !localized || localized === code ? fallback : localized;
  }

  const countryOptions = createCountryOptions(locale);

  const regionOptions: GeoOption[] = allCountries.flatMap(
    ([englishCountryName, countryCode, regions]) => {
      const countryName = localizedCountryName(countryCode, englishCountryName);
      return regions.flatMap(([regionName, shortCode]) => {
        const normalizedShortCode = shortCode?.toUpperCase();
        if (
          !normalizedShortCode ||
          !regionCodePattern.test(normalizedShortCode)
        ) {
          return [];
        }
        const value = `${countryCode}-${normalizedShortCode}`;
        return [
          {
            value,
            label: `${regionName} · ${countryName}`,
            searchText: `${regionName} ${countryName} ${englishCountryName} ${value}`,
          },
        ];
      });
    },
  );

  return { countryOptions, regionOptions };
}
