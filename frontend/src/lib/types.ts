export type Role = 'admin' | 'member';

export interface User {
  id: number;
  name: string;
  color: string;
  role: Role;
  avatar_emoji: string;
  pin_is_default: boolean;
}

export interface WeatherCurrent {
  temperature: number;
  feels_like: number;
  humidity: number;
  weather_code: number;
  wind_speed: number;
  is_day: boolean;
  icon: string;
  description: string;
}

export interface WeatherDay {
  date: string;
  weather_code: number;
  temp_max: number;
  temp_min: number;
  precip_probability: number;
  sunrise: string;
  sunset: string;
  icon: string;
  description: string;
}

export interface WeatherLocation {
  name: string;
  region?: string;
  country?: string;
  latitude: number;
  longitude: number;
  timezone: string;
}

export interface WeatherHour {
  time: string;
  temperature: number;
  precip_probability: number;
  precipitation: number;
  weather_code: number;
  icon: string;
}

/** Wann fängt es an, wann hört es auf — die Frage vor jeder Radtour. */
export interface WeatherRain {
  now: boolean;
  starts_at?: string;
  ends_at?: string;
  dry_until?: string;
  /** true, wenn die Angabe aus Viertelstundenwerten stammt. */
  fine_grained: boolean;
}

export interface WeatherData {
  current: WeatherCurrent;
  rain?: WeatherRain;
  hourly?: WeatherHour[];
  forecast: WeatherDay[];
  location: WeatherLocation;
  updated: string;
  /** true when the last refresh failed and this is the cached copy. */
  stale: boolean;
}

export type EventRepeat = 'none' | 'weekly' | 'monthly' | 'yearly';

export interface CalendarEvent {
  id: string;
  title: string;
  description: string;
  location: string;
  start: string;
  end: string;
  all_day: boolean;
  recurring: boolean;
  calendar: string;
  color: string;
  /** true for appointments entered here; .ics events stay read-only. */
  editable?: boolean;
  /** database id of the underlying appointment, for edit and delete. */
  event_id?: number;
  repeat?: EventRepeat;
}

export interface EventDraft {
  title: string;
  description: string;
  location: string;
  date: string;
  start_time: string;
  end_time: string;
  all_day: boolean;
  repeat: EventRepeat;
  color: string;
}

export interface ShoppingItem {
  id: number;
  name: string;
  quantity: string;
  category: string;
  checked: boolean;
  user_id: number | null;
  created_at: string;
  updated_at: string;
}

export interface Note {
  id: number;
  title: string;
  content: string;
  tags: string[];
  pinned: boolean;
  /** null means the note is shared with the whole family. */
  owner_id: number | null;
  source_file?: string;
  created_at: string;
  updated_at: string;
}

export interface Chore {
  id: number;
  title: string;
  description: string;
  interval_days: number;
  points: number;
  rotate: boolean;
  /** rotate | person | everyone | nobody */
  assignment: string;
  assignee_id: number | null;
  last_done_at: string | null;
  next_due_at: string | null;
  created_at: string;
  updated_at: string;
  assignee_name: string;
  assignee_color: string;
  assignee_emoji: string;
  is_overdue: boolean;
  days_until_due: number;
  /** Steht die Aufgabe heute an? Nur dann lässt sie sich abhaken. */
  is_due: boolean;
  /** Wer zuletzt abgehakt hat — leer, solange es niemand getan hat. */
  last_done_by?: string;
}

export interface Badge {
  id: string;
  label: string;
  emoji: string;
  description: string;
}

/** One family member's standing on the leaderboard. */
export interface Score {
  id: number;
  name: string;
  color: string;
  avatar_emoji: string;

  total_points: number;
  this_week: number;
  today: number;
  activities: number;
  chore_count: number;
  shop_count: number;

  rank: number;
  week_rank: number;
  level: number;
  level_name: string;
  /** 0-100 within the current level. */
  level_progress: number;
  points_to_next: number;

  streak_days: number;
  last_active?: string;
  badges: Badge[];
}

export type PointSource = 'chore' | 'shopping' | 'bonus';

export interface Activity {
  id: number;
  user_id: number;
  user_name: string;
  user_emoji: string;
  source: PointSource;
  points: number;
  note: string;
  created_at: string;
}

export interface DeviceTarget {
  id: number;
  name: string;
  type: 'http' | 'tcp';
  /** Address the health check probes — often an API path. */
  url: string;
  /** Web UI opened when the tile is tapped. */
  link: string;
  host: string;
  port: number;
  expect_status: number;
  icon: string;
  position: number;
  enabled: boolean;
}

export interface DeviceStatus extends DeviceTarget {
  status: 'up' | 'down' | 'unknown';
  latency_ms: number;
  last_check: string;
  error?: string;
}

export interface Link {
  id: number;
  owner_id: number | null;
  owner_name?: string;
  title: string;
  url: string;
  description: string;
  category: string;
  emoji: string;
  pinned: boolean;
  shared: boolean;
  position: number;
  /** false for a link shared by someone else. */
  editable: boolean;
}

export interface LinkDraft {
  title: string;
  url: string;
  description: string;
  category: string;
  emoji: string;
  pinned: boolean;
  shared: boolean;
}

/** Which dashboard widgets a person shows, and in what order. */
export interface DashboardLayout {
  order: string[];
  hidden: string[];
}

export interface Photo {
  name: string;
  size: number;
  modified: string;
}

export interface PhotoUploadResult {
  uploaded: string[];
  skipped?: Record<string, string>;
  count: number;
}

export interface BackupFile {
  name: string;
  size: number;
  modified: string;
}

export type ShoppingEvent =
  | { action: 'created' | 'updated' | 'deleted'; item: ShoppingItem }
  | { action: 'cleared'; item: ShoppingItem };
