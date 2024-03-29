// Copyright 2020 the Exposure Notifications Verification Server authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package database

var Countries = map[string]string{
	"Afghanistan":                      "af",
	"Aland Islands":                    "ax",
	"Albania":                          "al",
	"Algeria":                          "dz",
	"American Samoa":                   "as",
	"Andorra":                          "ad",
	"Angola":                           "ao",
	"Anguilla":                         "ai",
	"Antigua and Barbuda":              "ag",
	"Argentina":                        "ar",
	"Armenia":                          "am",
	"Aruba":                            "aw",
	"Australia":                        "au",
	"Austria":                          "at",
	"Azerbaijan":                       "az",
	"Bahamas":                          "bs",
	"Bahrain":                          "bh",
	"Bangladesh":                       "bd",
	"Barbados":                         "bb",
	"Belarus":                          "by",
	"Belgium":                          "be",
	"Belize":                           "bz",
	"Benin":                            "bj",
	"Bermuda":                          "bm",
	"Bhutan":                           "bt",
	"Bolivia":                          "bo",
	"Bosnia and Herzegovina":           "ba",
	"Botswana":                         "bw",
	"Brazil":                           "br",
	"British Indian Ocean Territory":   "io",
	"British Virgin Islands":           "vg",
	"Brunei":                           "bn",
	"Bulgaria":                         "bg",
	"Burkina Faso":                     "bf",
	"Burundi":                          "bi",
	"Cambodia":                         "kh",
	"Cameroon":                         "cm",
	"Canada":                           "ca",
	"Cape Verde":                       "cv",
	"Caribbean Netherlands":            "bq",
	"Cayman Islands":                   "ky",
	"Central African Republic":         "cf",
	"Chad":                             "td",
	"Chile":                            "cl",
	"China":                            "cn",
	"Christmas Island":                 "cx",
	"Cocos  Islands":                   "cc",
	"Colombia":                         "co",
	"Comoros":                          "km",
	"Congo":                            "cd",
	"Cook Islands":                     "ck",
	"Costa Rica":                       "cr",
	"Côte d’Ivoire":                    "ci",
	"Croatia":                          "hr",
	"Cuba":                             "cu",
	"Curaçao":                          "cw",
	"Cyprus":                           "cy",
	"Czech Republic":                   "cz",
	"Denmark":                          "dk",
	"Djibouti":                         "dj",
	"Dominica":                         "dm",
	"Dominican Republic":               "do",
	"Ecuador":                          "ec",
	"Egypt":                            "eg",
	"El Salvador":                      "sv",
	"Equatorial Guinea":                "gq",
	"Eritrea":                          "er",
	"Estonia":                          "ee",
	"Ethiopia":                         "et",
	"Falkland Islands":                 "fk",
	"Faroe Islands":                    "fo",
	"Fiji":                             "fj",
	"Finland":                          "fi",
	"France":                           "fr",
	"French Guiana":                    "gf",
	"French Polynesia":                 "pf",
	"Gabon":                            "ga",
	"Gambia":                           "gm",
	"Georgia":                          "ge",
	"Germany":                          "de",
	"Ghana":                            "gh",
	"Gibraltar":                        "gi",
	"Greece":                           "gr",
	"Greenland":                        "gl",
	"Grenada":                          "gd",
	"Guadeloupe":                       "gp",
	"Guam":                             "gu",
	"Guatemala":                        "gt",
	"Guernsey":                         "gg",
	"Guinea-Bissau":                    "gw",
	"Guinea":                           "gn",
	"Guyana":                           "gy",
	"Haiti":                            "ht",
	"Honduras":                         "hn",
	"Hong Kong":                        "hk",
	"Hungary":                          "hu",
	"Iceland":                          "is",
	"India":                            "in",
	"Indonesia":                        "id",
	"Iran":                             "ir",
	"Iraq":                             "iq",
	"Ireland":                          "ie",
	"Isle of Man":                      "im",
	"Israel":                           "il",
	"Italy":                            "it",
	"Jamaica":                          "jm",
	"Japan":                            "jp",
	"Jersey":                           "je",
	"Jordan":                           "jo",
	"Kazakhstan":                       "kz",
	"Kenya":                            "ke",
	"Kiribati":                         "ki",
	"Kosovo":                           "xk",
	"Kuwait":                           "kw",
	"Kyrgyzstan":                       "kg",
	"Laos":                             "la",
	"Latvia":                           "lv",
	"Lebanon":                          "lb",
	"Lesotho":                          "ls",
	"Liberia":                          "lr",
	"Libya":                            "ly",
	"Liechtenstein":                    "li",
	"Lithuania":                        "lt",
	"Luxembourg":                       "lu",
	"Macau":                            "mo",
	"Macedonia":                        "mk",
	"Madagascar":                       "mg",
	"Malawi":                           "mw",
	"Malaysia":                         "my",
	"Maldives":                         "mv",
	"Mali":                             "ml",
	"Malta":                            "mt",
	"Marshall Islands":                 "mh",
	"Martinique":                       "mq",
	"Mauritania":                       "mr",
	"Mauritius":                        "mu",
	"Mayotte":                          "yt",
	"Mexico":                           "mx",
	"Micronesia":                       "fm",
	"Moldova":                          "md",
	"Monaco":                           "mc",
	"Mongolia":                         "mn",
	"Montenegro":                       "me",
	"Montserrat":                       "ms",
	"Morocco":                          "ma",
	"Mozambique":                       "mz",
	"Myanmar":                          "mm",
	"Namibia":                          "na",
	"Nauru":                            "nr",
	"Nepal":                            "np",
	"Netherlands":                      "nl",
	"New Caledonia":                    "nc",
	"New Zealand":                      "nz",
	"Nicaragua":                        "ni",
	"Niger":                            "ne",
	"Nigeria":                          "ng",
	"Niue":                             "nu",
	"Norfolk Island":                   "nf",
	"North Korea":                      "kp",
	"Northern Mariana Islands":         "mp",
	"Norway":                           "no",
	"Oman":                             "om",
	"Pakistan":                         "pk",
	"Palau":                            "pw",
	"Palestine":                        "ps",
	"Panama":                           "pa",
	"Papua New Guinea":                 "pg",
	"Paraguay":                         "py",
	"Peru":                             "pe",
	"Philippines":                      "ph",
	"Poland":                           "pl",
	"Portugal":                         "pt",
	"Puerto Rico":                      "pr",
	"Qatar":                            "qa",
	"Réunion":                          "re",
	"Romania":                          "ro",
	"Russia":                           "ru",
	"Rwanda":                           "rw",
	"Saint Barthélemy":                 "bl",
	"Saint Helena":                     "sh",
	"Saint Kitts and Nevis":            "kn",
	"Saint Lucia":                      "lc",
	"Saint Martin":                     "mf",
	"Saint Pierre and Miquelon":        "pm",
	"Saint Vincent and the Grenadines": "vc",
	"Samoa":                            "ws",
	"San Marino":                       "sm",
	"São Tomé and Príncipe":            "st",
	"Saudi Arabia":                     "sa",
	"Senegal":                          "sn",
	"Serbia":                           "rs",
	"Seychelles":                       "sc",
	"Sierra Leone":                     "sl",
	"Singapore":                        "sg",
	"Sint Maarten":                     "sx",
	"Slovakia":                         "sk",
	"Slovenia":                         "si",
	"Solomon Islands":                  "sb",
	"Somalia":                          "so",
	"South Africa":                     "za",
	"South Korea":                      "kr",
	"South Sudan":                      "ss",
	"Spain":                            "es",
	"Sri Lanka":                        "lk",
	"Sudan":                            "sd",
	"Suriname":                         "sr",
	"Svalbard and Jan Mayen":           "sj",
	"Swaziland":                        "sz",
	"Sweden":                           "se",
	"Switzerland":                      "ch",
	"Syria":                            "sy",
	"Taiwan":                           "tw",
	"Tajikistan":                       "tj",
	"Tanzania":                         "tz",
	"Thailand":                         "th",
	"Timor-Leste":                      "tl",
	"Togo":                             "tg",
	"Tokelau":                          "tk",
	"Tonga":                            "to",
	"Trinidad and Tobago":              "tt",
	"Tunisia":                          "tn",
	"Turkey":                           "tr",
	"Turkmenistan":                     "tm",
	"Turks and Caicos Islands":         "tc",
	"Tuvalu":                           "tv",
	"U.S. Virgin Islands":              "vi",
	"Uganda":                           "ug",
	"Ukraine":                          "ua",
	"United Arab Emirates":             "ae",
	"United Kingdom":                   "gb",
	"United States":                    "us",
	"Uruguay":                          "uy",
	"Uzbekistan":                       "uz",
	"Vanuatu":                          "vu",
	"Vatican City":                     "va",
	"Venezuela":                        "ve",
	"Vietnam":                          "vn",
	"Wallis and Futuna":                "wf",
	"Western Sahara":                   "eh",
	"Yemen":                            "ye",
	"Zambia":                           "zm",
	"Zimbabwe":                         "zw",
}
