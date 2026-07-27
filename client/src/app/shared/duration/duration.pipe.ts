import { Pipe, PipeTransform } from '@angular/core';
import { DurationFormatter } from './duration.formatter';
import { Duration } from 'date-fns';

@Pipe({
    name: 'duration',
    standalone: false
})
export class DurationPipe implements PipeTransform {
    transform(value: Duration): string {
        return DurationFormatter.durationToString(value);
    }
}
