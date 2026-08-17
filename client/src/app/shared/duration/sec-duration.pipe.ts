import { Pipe, PipeTransform } from '@angular/core';
import { DurationConverter } from './duration.converter';
import { DurationFormatter } from './duration.formatter';

@Pipe({
    name: 'secDuration',
    standalone: false
})
export class SecDurationPipe implements PipeTransform {
    public transform(value: number): string {
        return DurationFormatter.durationToString(DurationConverter.secondsToDuration(value));
    }
}
