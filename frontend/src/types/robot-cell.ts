import { CellState } from './enums/validation-status';

export interface RobotCell {
  id: number;
  cell_code: string;
  name: string;
  layout_geojson: GeoJSONValue;
  robot_model: string;
  controller_model: string;
  max_reach_mm: number;
  owner_team: string;
  cell_state: CellState;
  layout_version: number;
  zone_count: number;
  program_count: number;
  created_at: string;
  updated_at: string;
}

export type GeoJSONValue = Record<string, unknown>;

export interface CreateRobotCell {
  cell_code: string;
  name: string;
  layout_geojson: GeoJSONValue;
  robot_model: string;
  controller_model: string;
  max_reach_mm: number;
  owner_team: string;
}

export interface UpdateRobotCell extends Omit<CreateRobotCell, 'cell_code'> {
  layout_version: number;
}
