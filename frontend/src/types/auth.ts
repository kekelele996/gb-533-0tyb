export type Role = 'admin' | 'robot_programmer' | 'safety_engineer' | 'reviewer' | 'auditor';

export interface UserView {
  id: number;
  username: string;
  role: Role;
}

export interface LoginResponse {
  token: string;
  expires_at: string;
  user: UserView;
}
