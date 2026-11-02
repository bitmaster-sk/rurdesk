import { FormControl } from '@angular/forms';
import { ReleaseOpenIssues } from '../constants/release-open-issues.enum';

export interface ReleaseForm {
    name: FormControl<string>;
    recreateTag: FormControl<boolean>;
    openIssues: FormControl<ReleaseOpenIssues>;
    notes: FormControl<string>;
}
