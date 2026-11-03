export abstract class CostFormatter {
    public static format(costUsd: number | null, estimated: boolean): string | null {
        if (costUsd === null) {
            return null;
        }
        const value = costUsd < 0.01 ? '<$0.01' : `$${costUsd.toFixed(2)}`;
        return estimated ? `≈ ${value}` : value;
    }
}
