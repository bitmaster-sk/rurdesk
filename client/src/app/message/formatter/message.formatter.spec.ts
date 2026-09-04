import { MessageFormatter } from './message.formatter';

describe('MessageFormatter.formatUnreadBadgeText', () => {
    it.each([
        [0, undefined],
        [-3, undefined],
        [1, '1'],
        [99, '99'],
        [100, '99+'],
        [150, '99+']
    ])('formats %s as %s', (count, expected) => {
        expect(MessageFormatter.formatUnreadBadgeText(count)).toBe(expected);
    });
});
