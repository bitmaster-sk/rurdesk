import { ChartDataset } from 'chart.js';

export interface TrackedTimeChartDataset extends ChartDataset<'bar'> {
    idIssuePublic: number;
    issueTitle: string;
}
