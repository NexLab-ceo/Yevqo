let preferences={locale:'ru',timezone:'UTC',date_format:'locale'};
export function setDatePreferences(next={}){preferences={...preferences,...next};}
export function formatDate(value){if(!value)return '—';const date=new Date(value);if(!Number.isFinite(date.getTime()))return '—';const locale=preferences.date_format==='iso'?'sv-SE':(preferences.locale==='ru'?'ru-RU':'en-US');try{return new Intl.DateTimeFormat(locale,{dateStyle:'medium',timeStyle:'short',timeZone:preferences.timezone||'UTC'}).format(date)}catch{return date.toLocaleString()}}
