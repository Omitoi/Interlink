export type CityOption = { name: string; lat: number; lon: number };

export const FI_CITIES: readonly CityOption[] = [
  { name: "Helsinki",     lat: 60.1699, lon: 24.9384 },
  { name: "Espoo",        lat: 60.2055, lon: 24.6559 },
  { name: "Tampere",      lat: 61.4978, lon: 23.7610 },
  { name: "Vantaa",       lat: 60.2934, lon: 25.0378 },
  { name: "Oulu",         lat: 65.0121, lon: 25.4651 },
  { name: "Turku",        lat: 60.4518, lon: 22.2666 },
  { name: "Jyväskylä",    lat: 62.2426, lon: 25.7473 },
  { name: "Kuopio",       lat: 62.8924, lon: 27.6770 },
  { name: "Lahti",        lat: 60.9827, lon: 25.6615 },
  { name: "Pori",         lat: 61.4850, lon: 21.7970 },
  { name: "Kouvola",      lat: 60.8681, lon: 26.7042 },
  { name: "Joensuu",      lat: 62.6010, lon: 29.7630 },
  { name: "Lappeenranta", lat: 61.0583, lon: 28.1887 },
  { name: "Hämeenlinna",  lat: 60.9969, lon: 24.4643 },
  { name: "Vaasa",        lat: 63.0951, lon: 21.6158 },
  { name: "Rovaniemi",    lat: 66.5039, lon: 25.7294 },
  { name: "Seinäjoki",    lat: 62.7903, lon: 22.8403 },
  { name: "Mikkeli",      lat: 61.6886, lon: 27.2722 },
] as const;
