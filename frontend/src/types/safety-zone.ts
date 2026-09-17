import { GeoJSONValue } from './robot-cell';
import { ZoneState, ZoneType } from './enums/zone-type';

export interface SafetyZone {
  id: number;
  robot_cell_id: number;
  robot_cell_code: string;
  name: string;
  zone_type: ZoneType;
  polygon_geojson: GeoJSONValue;
  min_height_mm: number;
  max_height_mm: number;
  speed_limit_mm_s: number;
  access_rule: string;
  zone_state: ZoneState;
  version: number;
  created_at: string;
  updated_at: string;
}

export interface CreateSafetyZone {
  robot_cell_id: number;
  name: string;
  zone_type: ZoneType;
  polygon_geojson: GeoJSONValue;
  min_height_mm: number;
  max_height_mm: number;
  speed_limit_mm_s: number;
  access_rule: string;
}

export interface UpdateSafetyZone extends Omit<CreateSafetyZone, 'robot_cell_id'> {
  version: number;
}
