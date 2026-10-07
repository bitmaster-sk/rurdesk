import { WikiBudgetCheck } from '../entity/wiki-budget-check.entity';
import { WikiBudgetConverter } from './wiki-budget.converter';

const overLimit: WikiBudgetCheck = {
    isAlways: true,
    wasAlways: true,
    isShared: false,
    ownChars: 4000,
    previousChars: 3000,
    usedTokens: 1200,
    limit: 1000
};

describe('WikiBudgetConverter', () => {
    it('blocks an always page that grows past the limit', () => {
        expect(WikiBudgetConverter.toIsOverBudget(overLimit)).toBe(true);
    });

    it('lets an always page over the limit shrink or stay the same size', () => {
        expect(WikiBudgetConverter.toIsOverBudget({ ...overLimit, ownChars: 2000 })).toBe(false);
        expect(WikiBudgetConverter.toIsOverBudget({ ...overLimit, ownChars: 3000 })).toBe(false);
    });

    it('checks a page that becomes always even when it is small', () => {
        expect(
            WikiBudgetConverter.toIsOverBudget({ ...overLimit, wasAlways: false, ownChars: 10 })
        ).toBe(true);
    });

    it('ignores pages that are not always and shared pages seen from a project with limit 0', () => {
        expect(WikiBudgetConverter.toIsOverBudget({ ...overLimit, isAlways: false })).toBe(false);
        expect(WikiBudgetConverter.toIsOverBudget({ ...overLimit, isShared: true, limit: 0 })).toBe(
            false
        );
        expect(WikiBudgetConverter.toIsOverBudget({ ...overLimit, limit: 0 })).toBe(true);
    });
});
