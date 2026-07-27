import { Pipe, PipeTransform } from '@angular/core';
import { fromAsciiToEmoji } from './ascii-emoji';

declare var joypixels: {
    shortnameToUnicode(input: string): string;
};

@Pipe({
    name: 'emoji',
    standalone: false
})
export class EmojiPipe implements PipeTransform {
    transform(value: string): string {
        return fromAsciiToEmoji(joypixels.shortnameToUnicode(value));
    }
}
