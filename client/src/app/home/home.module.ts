import { NgModule } from '@angular/core';
import { HomeRoutingModule } from './home-routing.module';
import { CoreModule } from '../core/core.module';
import { MessageModule } from '../message/message.module';
import { AgentChatMenuModule } from '../agent-chat-menu/agent-chat-menu.module';
import { SharedModule } from '../shared/shared.module';
import { NgClickOutsideDirective } from 'ng-click-outside2';
import { ProjectModule } from '../project/project.module';
import { NotificationModule } from '../notification/notification.module';
import { LicenseBannerComponent } from './components/license-banner/license-banner.component';
import { ShellLayoutComponent } from './layouts/shell-layout/shell-layout.component';
import { SideMenuComponent } from './components/side-menu/side-menu.component';
import { ProjectSwitcherComponent } from './components/project-switcher/project-switcher.component';
import { UserMenuComponent } from './components/user-menu/user-menu.component';
import {
    TablerIconComponent,
    provideTablerIcons,
    IconPlus,
    IconUser,
    IconUsers,
    IconSettings,
    IconLogout,
    IconTable,
    IconLayoutColumns,
    IconCalendar,
    IconChartColumn,
    IconDeviceDesktop,
    IconBook2,
    IconRocket,
    IconSparkles,
    IconInfoCircle,
    IconAlertTriangle,
    IconBan,
    IconX,
    IconSearch,
    IconSelector,
    IconCheck,
    IconBell,
    IconMessage,
    IconRobot,
    IconKeyboard,
    IconShield,
    IconLayoutSidebarLeftCollapse,
    IconLayoutSidebarLeftExpand
} from '@tabler/icons-angular';

@NgModule({
    declarations: [
        LicenseBannerComponent,
        ShellLayoutComponent,
        SideMenuComponent,
        ProjectSwitcherComponent,
        UserMenuComponent
    ],
    imports: [
        CoreModule,
        SharedModule,
        HomeRoutingModule,
        MessageModule,
        AgentChatMenuModule,
        ProjectModule,
        NgClickOutsideDirective,
        NotificationModule,
        TablerIconComponent
    ],
    providers: [
        provideTablerIcons({
            IconPlus,
            IconUser,
            IconBook2,
            IconRocket,
            IconUsers,
            IconSettings,
            IconLogout,
            IconDeviceDesktop,
            IconTable,
            IconLayoutColumns,
            IconCalendar,
            IconChartColumn,
            IconSparkles,
            IconInfoCircle,
            IconAlertTriangle,
            IconBan,
            IconX,
            IconSearch,
            IconSelector,
            IconCheck,
            IconBell,
            IconMessage,
            IconRobot,
            IconKeyboard,
            IconShield,
            IconLayoutSidebarLeftCollapse,
            IconLayoutSidebarLeftExpand
        })
    ]
})
export class HomeModule {}
