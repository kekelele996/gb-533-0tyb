import { inject } from '@angular/core';
import { ValidationRunStore } from '../stores/validation-run.store';

export function useValidationRun(): ValidationRunStore {
  return inject(ValidationRunStore);
}
