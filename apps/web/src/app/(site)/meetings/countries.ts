// Country dialling codes for the phone field.
//
// +1 is first and is the default, because the owner is in the United
// States and that is what almost every caller will need. The rest exist
// so a member abroad is not stuck: this is a hiring conversation, and
// someone in London should not have to write their number in a comments
// box because a form assumed North America.
//
// Not exhaustive on purpose. A 200-entry list is worse to use than a
// short one covering where a hiring manager plausibly calls from, and
// anyone outside it still has the "my number is in the comments"
// escape hatch.
export type Country = { code: string; label: string };

export const COUNTRY_CODES: Country[] = [
  { code: "+1", label: "+1 United States and Canada" },
  { code: "+44", label: "+44 United Kingdom" },
  { code: "+353", label: "+353 Ireland" },
  { code: "+61", label: "+61 Australia" },
  { code: "+64", label: "+64 New Zealand" },
  { code: "+49", label: "+49 Germany" },
  { code: "+33", label: "+33 France" },
  { code: "+34", label: "+34 Spain" },
  { code: "+39", label: "+39 Italy" },
  { code: "+31", label: "+31 Netherlands" },
  { code: "+32", label: "+32 Belgium" },
  { code: "+41", label: "+41 Switzerland" },
  { code: "+43", label: "+43 Austria" },
  { code: "+45", label: "+45 Denmark" },
  { code: "+46", label: "+46 Sweden" },
  { code: "+47", label: "+47 Norway" },
  { code: "+48", label: "+48 Poland" },
  { code: "+351", label: "+351 Portugal" },
  { code: "+420", label: "+420 Czechia" },
  { code: "+52", label: "+52 Mexico" },
  { code: "+55", label: "+55 Brazil" },
  { code: "+54", label: "+54 Argentina" },
  { code: "+56", label: "+56 Chile" },
  { code: "+57", label: "+57 Colombia" },
  { code: "+27", label: "+27 South Africa" },
  { code: "+972", label: "+972 Israel" },
  { code: "+971", label: "+971 United Arab Emirates" },
  { code: "+90", label: "+90 Turkey" },
  { code: "+91", label: "+91 India" },
  { code: "+86", label: "+86 China" },
  { code: "+81", label: "+81 Japan" },
  { code: "+82", label: "+82 South Korea" },
  { code: "+65", label: "+65 Singapore" },
  { code: "+852", label: "+852 Hong Kong" },
  { code: "+60", label: "+60 Malaysia" },
  { code: "+66", label: "+66 Thailand" },
  { code: "+62", label: "+62 Indonesia" },
  { code: "+63", label: "+63 Philippines" },
];

export const DEFAULT_COUNTRY = "+1";

// What to ask for, which is not the same everywhere. North American
// numbers are always ten digits, so saying so catches a typo. Elsewhere
// the length varies by country and a fixed rule would reject real
// numbers, so the hint is honest about being looser.
export function numberHint(code: string) {
  return code === "+1"
    ? "Area code and number, ten digits."
    : "Digits only, no spaces or punctuation.";
}
