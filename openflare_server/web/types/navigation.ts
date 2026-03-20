export type NavigationIconKey =
  | 'home'
  | 'node'
  | 'website'
  | 'origin'
  | 'domain'
  | 'certificate'
  | 'proxy'
  | 'release'
  | 'log'
  | 'performance'
  | 'user'
  | 'setting';

export interface NavigationItem {
  href: string;
  label: string;
  icon: NavigationIconKey;
  children?: NavigationItem[];
}
