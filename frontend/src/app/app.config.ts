import { ApplicationConfig, importProvidersFrom } from '@angular/core';
import { provideRouter } from '@angular/router';
import { provideHttpClient, withInterceptors } from '@angular/common/http';
import { provideAnimationsAsync } from '@angular/platform-browser/animations/async';
import {
  AlertTriangle, Boxes, ChevronRight, CircleCheck, CircleDot, CircleOff, ClipboardCheck,
  DoorOpen, FileCode2, GitBranch, Grid3X3, Info, LogIn, LogOut, LucideAngularModule,
  Map, PauseCircle, Play, Plus, RefreshCw, RotateCcw, Save, ScanLine, ShieldCheck,
  Snowflake, TriangleAlert, Upload, UserCheck, X,
} from 'lucide-angular';
import { routes } from '../router/app.routes';
import { authInterceptor } from '../api/auth.interceptor';

export const appConfig: ApplicationConfig = {
  providers: [
    provideRouter(routes),
    provideHttpClient(withInterceptors([authInterceptor])),
    provideAnimationsAsync(),
    importProvidersFrom(LucideAngularModule.pick({
      AlertTriangle, Boxes, ChevronRight, CircleCheck, CircleDot, CircleOff, ClipboardCheck,
      DoorOpen, FileCode2, GitBranch, Grid3X3, Info, LogIn, LogOut, Map, PauseCircle,
      Play, Plus, RefreshCw, RotateCcw, Save, ScanLine, ShieldCheck, Snowflake,
      TriangleAlert, Upload, UserCheck, X,
    })),
  ],
};
