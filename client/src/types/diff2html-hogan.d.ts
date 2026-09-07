/* Required because skipLibCheck is off: diff2html's .d.ts imports this untyped
   transitive dependency, which has no @types package. Only the members diff2html
   references are declared. */
declare module '@profoundlogic/hogan' {
    export type Context = Record<string, unknown>;

    export type Partials = Record<string, Template>;

    export interface Template {
        render(context?: Context, partials?: Partials, indent?: string): string;
    }

    export function compile(text: string, options?: Record<string, unknown>): Template;
}
