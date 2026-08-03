import { A11yModule } from '@angular/cdk/a11y';
import { OverlayModule } from '@angular/cdk/overlay';
import { NgModule } from '@angular/core';
import { ReactiveFormsModule } from '@angular/forms';
import { RouterModule } from '@angular/router';
import {
    IconAlertCircle,
    IconAlertTriangle,
    IconArrowRight,
    IconCalendar,
    IconChartBar,
    IconChartColumn,
    IconChevronDown,
    IconCircleCheck,
    IconCommand,
    IconCopy,
    IconFlag,
    IconHome,
    IconInfoCircle,
    IconKeyboard,
    IconLayoutBoard,
    IconLayoutColumns,
    IconLink,
    IconList,
    IconLoader2,
    IconLogout,
    IconPlus,
    IconSearch,
    IconSettings,
    IconTable,
    IconUser,
    IconX,
    TablerIconComponent,
    provideTablerIcons
} from '@tabler/icons-angular';
import { CoreModule } from '../core/core.module';
import { UiCommandPaletteComponent } from './components/command-palette/command-palette.component';
import { UiCommandHelpComponent } from './components/command-help/command-help.component';
import { UiMenuComponent } from './components/menu/menu.component';
import { UiBadgeComponent } from './components/badge/badge.component';
import { UiOdometerComponent } from './components/odometer/odometer.component';
import { UiButtonComponent } from './components/button/button.component';
import { UiChoiceComponent } from './components/choice/choice.component';
import { UiConfirmPopupComponent } from './components/confirm-popup/confirm-popup.component';
import { UiDialogComponent } from './components/dialog/dialog.component';
import { UiDividerComponent } from './components/divider/divider.component';
import { UiLoaderComponent } from './components/loader/loader.component';
import { UiMessageComponent } from './components/message/message.component';
import { UiPopoverComponent } from './components/popover/popover.component';
import { UiSaveStatusChipComponent } from './components/save-status/save-status-chip.component';
import { UiSaveStatusDirective } from './directives/save-status.directive';
import { UiFlipListDirective } from './directives/flip-list.directive';
import { UiListboxComponent } from './components/select/listbox.component';
import { UiMultiSelectComponent } from './components/select/multiselect.component';
import { UiOptionPanelComponent } from './components/select/option-panel.component';
import { UiDateRangeSelectComponent } from './components/date-range-select/date-range-select.component';
import { UiSelectComponent } from './components/select/select.component';
import { UiTagComponent } from './components/tag/tag.component';
import { UiToggleButtonComponent } from './components/toggle-button/toggle-button.component';
import { UiTooltipComponent } from './components/tooltip/tooltip.component';
import { UiTableSortDirective } from './components/table/table-sort.directive';
import { UiSortColumnDirective } from './components/table/sort-column.directive';
import { UiCheckboxDirective } from './directives/checkbox.directive';
import { UiColorDirective } from './directives/color.directive';
import { UiConfirmDirective } from './directives/confirm.directive';
import { UiDatepickerDirective } from './directives/datepicker.directive';
import { UiInputDirective } from './directives/input.directive';
import { UiTextareaDirective } from './directives/textarea.directive';
import { UiToggleDirective } from './directives/toggle.directive';
import { UiTooltipDirective } from './directives/tooltip.directive';

@NgModule({
    declarations: [
        UiCommandPaletteComponent,
        UiCommandHelpComponent,
        UiButtonComponent,
        UiConfirmPopupComponent,
        UiDividerComponent,
        UiLoaderComponent,
        UiMessageComponent,
        UiTagComponent,
        UiBadgeComponent,
        UiOdometerComponent,
        UiOptionPanelComponent,
        UiSelectComponent,
        UiDateRangeSelectComponent,
        UiMultiSelectComponent,
        UiListboxComponent,
        UiChoiceComponent,
        UiToggleButtonComponent,
        UiPopoverComponent,
        UiMenuComponent,
        UiTooltipComponent,
        UiDialogComponent,
        UiSaveStatusChipComponent,
        UiTableSortDirective,
        UiSortColumnDirective,
        UiConfirmDirective,
        UiSaveStatusDirective,
        UiFlipListDirective,
        UiInputDirective,
        UiTextareaDirective,
        UiCheckboxDirective,
        UiColorDirective,
        UiToggleDirective,
        UiDatepickerDirective,
        UiTooltipDirective
    ],
    imports: [
        ReactiveFormsModule,
        CoreModule,
        OverlayModule,
        A11yModule,
        RouterModule,
        TablerIconComponent
    ],
    exports: [
        UiCommandPaletteComponent,
        UiCommandHelpComponent,
        UiConfirmDirective,
        UiDividerComponent,
        UiLoaderComponent,
        UiMessageComponent,
        UiButtonComponent,
        UiTagComponent,
        UiBadgeComponent,
        UiOdometerComponent,
        UiSelectComponent,
        UiDateRangeSelectComponent,
        UiMultiSelectComponent,
        UiListboxComponent,
        UiChoiceComponent,
        UiToggleButtonComponent,
        UiPopoverComponent,
        UiMenuComponent,
        UiDialogComponent,
        UiTableSortDirective,
        UiSortColumnDirective,
        UiSaveStatusDirective,
        UiFlipListDirective,
        UiInputDirective,
        UiTextareaDirective,
        UiCheckboxDirective,
        UiColorDirective,
        UiToggleDirective,
        UiDatepickerDirective,
        UiTooltipDirective
    ],
    providers: [
        provideTablerIcons({
            IconChevronDown,
            IconLoader2,
            IconX,
            IconInfoCircle,
            IconCircleCheck,
            IconAlertTriangle,
            IconAlertCircle,
            IconCommand,
            IconCopy,
            IconArrowRight,
            IconUser,
            IconFlag,
            IconLink,
            IconSettings,
            IconPlus,
            IconSearch,
            IconList,
            IconTable,
            IconLayoutBoard,
            IconLayoutColumns,
            IconCalendar,
            IconChartBar,
            IconChartColumn,
            IconLogout,
            IconHome,
            IconKeyboard
        })
    ]
})
export class UiModule {}
