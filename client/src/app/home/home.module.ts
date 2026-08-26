import { NgModule } from '@angular/core';
import { HomeRoutingModule } from './home-routing.module';
import { CoreModule } from '../core/core.module';
import { MessageModule } from '../message/message.module';
import { SharedModule } from '../shared/shared.module';
import { NgClickOutsideDirective } from 'ng-click-outside2';
import { ProjectModule } from '../project/project.module';
import { FormsModule } from '@angular/forms';
import { NotificationModule } from '../notification/notification.module';
import { TopMenuComponent } from './components/top-menu/top-menu.component';
import { AppLayoutComponent } from './layouts/app-layout/app-layout.component';
import { ProjectLayoutComponent } from './layouts/project-layout/project-layout.component';
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
    IconSparkles
} from '@tabler/icons-angular';

@NgModule({
    declarations: [TopMenuComponent, AppLayoutComponent, ProjectLayoutComponent],
    imports: [
        CoreModule,
        SharedModule,
        HomeRoutingModule,
        MessageModule,
        ProjectModule,
        NgClickOutsideDirective,
        FormsModule,
        NotificationModule,
        TablerIconComponent
    ],
    providers: [
        provideTablerIcons({
            IconPlus,
            IconUser,
            IconUsers,
            IconSettings,
            IconLogout,
            IconDeviceDesktop,
            IconTable,
            IconLayoutColumns,
            IconCalendar,
            IconChartColumn,
            IconSparkles
        })
    ]
})
export class HomeModule {}
