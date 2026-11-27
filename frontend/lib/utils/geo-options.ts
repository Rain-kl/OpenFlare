import { allCountries } from 'country-region-data';

export interface GeoOption {
  value: string;
  label: string;
  searchText: string;
}

const countryCodePattern = /^[A-Z]{2}$/;

export function createCountryOptions(locale: string): GeoOption[] {
  const countryDisplayNames = new Intl.DisplayNames([locale], {
    type: 'region',
  });

  function localizedCountryName(code: string, fallback: string) {
    const localized = countryDisplayNames.of(code);
    return !localized || localized === code ? fallback : localized;
  }

  return allCountries
    .filter(([, code]) => countryCodePattern.test(code))
    .map(([englishName, code]) => {
      const label = localizedCountryName(code, englishName);
      return {
        value: code,
        label,
        searchText: `${label} ${englishName} ${code}`,
      };
    })
    .sort((left, right) => left.label.localeCompare(right.label, locale));
}
