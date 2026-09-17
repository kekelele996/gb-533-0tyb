import { ProgramState } from './enums/validation-status';

export interface TrajectoryPoint {
  x_mm: number;
  y_mm: number;
  z_mm: number;
  time_ms: number;
  speed_mm_s?: number;
}

export interface InterlockEvent {
  name: string;
  sequence: number;
  depends_on: string[];
}

export interface MotionProgram {
  id: number;
  robot_cell_id: number;
  robot_cell_code: string;
  program_code: string;
  version: number;
  trajectory: TrajectoryPoint[];
  tool_radius_mm: number;
  payload_radius_mm: number;
  interlock_sequence: InterlockEvent[];
  source_checksum: string;
  program_state: ProgramState;
  uploaded_by: number;
  uploaded_at: string;
}

export interface CreateMotionProgram {
  robot_cell_id: number;
  program_code: string;
  version: number;
  trajectory: TrajectoryPoint[];
  tool_radius_mm: number;
  payload_radius_mm: number;
  interlock_sequence: InterlockEvent[];
}
