import { WikiBudgetCheck } from '../entity/wiki-budget-check.entity';

export abstract class WikiBudgetConverter {
    public static toIsOverBudget(check: WikiBudgetCheck): boolean {
        if (!check.isAlways) {
            return false;
        }
        if (check.wasAlways && check.ownChars <= check.previousChars) {
            return false;
        }
        if (check.isShared && check.limit === 0) {
            return false;
        }
        return check.usedTokens > check.limit;
    }
}
