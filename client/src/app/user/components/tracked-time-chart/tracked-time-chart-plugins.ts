import { Chart, ChartDataset, Plugin } from 'chart.js';
import { TrackedTimeChartText } from './tracked-time-chart-text';

interface SegmentDataset {
    idIssuePublic: number;
    issueTitle: string;
}

export abstract class TrackedTimeChartPlugins {
    public static readonly segmentLabels: Plugin<'bar'> = {
        id: 'trackedTimeSegmentLabels',
        afterDatasetsDraw: (chart: Chart<'bar'>) => {
            const { ctx } = chart;
            const fontFamily = getComputedStyle(chart.canvas).fontFamily;
            const color = getComputedStyle(document.documentElement)
                .getPropertyValue('--ui-color-text')
                .trim();
            ctx.save();
            ctx.textAlign = 'left';
            ctx.textBaseline = 'middle';
            ctx.fillStyle = color;
            ctx.font = `700 11px ${fontFamily}`;
            const measure = (t: string): number => ctx.measureText(t).width;
            chart.data.datasets.forEach((dataset, datasetIndex) => {
                if (!chart.isDatasetVisible(datasetIndex)) {
                    return;
                }
                const { idIssuePublic, issueTitle } = dataset as ChartDataset<'bar'> &
                    Partial<SegmentDataset>;
                if (idIssuePublic === undefined || issueTitle === undefined) {
                    return;
                }
                const num = `#${idIssuePublic}`;
                chart.getDatasetMeta(datasetIndex).data.forEach(bar => {
                    const props = bar.getProps(['x', 'y', 'base', 'height'], true) as {
                        x: number;
                        y: number;
                        base: number;
                        height: number;
                    };
                    const left = Math.min(props.base, props.x);
                    const width = Math.abs(props.x - props.base);
                    const text = TrackedTimeChartText.segment(
                        width,
                        props.height,
                        num,
                        issueTitle,
                        measure
                    );
                    if (text === null) {
                        return;
                    }
                    ctx.font = `700 11px ${fontFamily}`;
                    ctx.fillText(num, left + 6, props.y);
                    if (text !== num) {
                        const numWidth = ctx.measureText(num).width;
                        ctx.font = `400 11px ${fontFamily}`;
                        ctx.fillText(
                            text.slice(num.length + 1),
                            left + 6 + numWidth + ctx.measureText(' ').width,
                            props.y
                        );
                    }
                });
            });
            ctx.restore();
        }
    };

    public static readonly dayTotals: Plugin<'bar'> = {
        id: 'trackedTimeDayTotals',
        afterDatasetsDraw: (chart: Chart<'bar'>) => {
            const labels = chart.data.labels;
            if (!labels) {
                return;
            }
            const { ctx } = chart;
            const fontFamily = getComputedStyle(chart.canvas).fontFamily;
            const color = getComputedStyle(document.documentElement)
                .getPropertyValue('--ui-color-text-muted')
                .trim();
            ctx.save();
            ctx.textAlign = 'left';
            ctx.textBaseline = 'middle';
            ctx.font = `700 11px ${fontFamily}`;
            ctx.fillStyle = color;
            labels.forEach((_, day) => {
                let total = 0;
                chart.data.datasets.forEach((dataset, datasetIndex) => {
                    if (!chart.isDatasetVisible(datasetIndex)) {
                        return;
                    }
                    const value = dataset.data[day];
                    if (typeof value === 'number') {
                        total += value;
                    }
                });
                if (total <= 0) {
                    return;
                }
                const x = chart.scales['x'].getPixelForValue(total) + 6;
                const y = chart.scales['y'].getPixelForValue(day);
                ctx.fillText(TrackedTimeChartText.hours(total), x, y);
            });
            ctx.restore();
        }
    };
}
