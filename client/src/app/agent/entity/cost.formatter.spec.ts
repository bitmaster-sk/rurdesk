import { CostFormatter } from './cost.formatter';

describe('CostFormatter.format', () => {
    it('returns null for an unknown cost', () => {
        expect(CostFormatter.format(null, false)).toBeNull();
        expect(CostFormatter.format(null, true)).toBeNull();
    });

    it('formats sub-cent costs as <$0.01', () => {
        expect(CostFormatter.format(0.004, false)).toBe('<$0.01');
        expect(CostFormatter.format(0, false)).toBe('<$0.01');
    });

    it('formats dollars with two decimals', () => {
        expect(CostFormatter.format(0.4213, false)).toBe('$0.42');
        expect(CostFormatter.format(12, false)).toBe('$12.00');
    });

    it('marks estimated costs with ≈', () => {
        expect(CostFormatter.format(0.4213, true)).toBe('≈ $0.42');
        expect(CostFormatter.format(0.004, true)).toBe('≈ <$0.01');
    });
});
