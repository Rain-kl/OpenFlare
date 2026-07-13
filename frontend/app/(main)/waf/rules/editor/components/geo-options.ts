import { allCountries } from 'country-region-data';

export interface GeoOption {
  value: string;
  label: string;
  searchText: string;
}

const countryCodePattern = /^[A-Z]{2}$/;
const regionCodePattern = /^[A-Z0-9]{1,3}$/;
const countryDisplayNames = new Intl.DisplayNames(['zh-CN'], {
  type: 'region',
});

function localizedCountryName(code: string, fallback: string) {
  const localized = countryDisplayNames.of(code);
  return !localized || localized === code ? fallback : localized;
}

export const countryOptions: GeoOption[] = allCountries
  .filter(([, code]) => countryCodePattern.test(code))
  .map(([englishName, code]) => {
    const label = localizedCountryName(code, englishName);
    return {
      value: code,
      label,
      searchText: `${label} ${englishName} ${code}`,
    };
  })
  .sort((left, right) => left.label.localeCompare(right.label, 'zh-CN'));

export const regionOptions: GeoOption[] = allCountries.flatMap(
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
