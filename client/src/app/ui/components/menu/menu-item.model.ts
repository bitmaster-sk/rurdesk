import { UiBadgeSeverity } from '../badge/badge.component';

/**
 * Menu model item for `<ui-menu>`.
 *
 * One-level grouping only: an item with `items` renders as a section header + a
 * flat sub-list (NOT a cascading fly-out). Group detection is by the
 * **presence** of `items` (an empty `items: []` renders header-only), not its
 * length.
 *
 * Constraint (track key): labels must be unique within a single menu — the
 * `trackItem` fn keys `command` items by `label`, so two same-label items would
 * throw NG0955 (duplicate track key). True across all current call-sites.
 */
export interface UiMenuItem {
    /** i18n key resolved at render time via `| translate` in the template.
     *  When present, the menu renders `labelKey | translate` instead of
     *  `label`, making the text reactive to language switches. Opt-in —
     *  items without `labelKey` fall back to `label`. */
    labelKey?: string;
    /** Visible text (also the track key for command items — must be unique). */
    label?: string;
    /** Tabler icon name, e.g. `'user'`. Rendered via `<tabler-icon>`; the icon
     *  must be registered with `provideTablerIcons` in the consuming module. */
    icon?: string;
    /** Router navigation target; rendered as an `<a role="menuitem" routerLink>`. */
    routerLink?: string | unknown[];
    /** Click callback; rendered as a `<button role="menuitem">`. Zero-arg. */
    command?: () => void;
    /** One-level group: header (`label`) + flat sub-list. Presence = group. */
    items?: UiMenuItem[];
    /** Renders a divider row instead of an item. */
    separator?: boolean;
    /** Count pill rendered via `<ui-badge>`. */
    badge?: string;
    /** Colour of the badge pill. Defaults to `secondary`. */
    badgeSeverity?: UiBadgeSeverity;
}
