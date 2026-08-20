import { MARKED_OPTIONS, MarkedOptions } from 'ngx-markdown';

const markedOptions: MarkedOptions = {
    breaks: true
};

export const MARKDOWN_MARKED_OPTIONS = {
    provide: MARKED_OPTIONS,
    useValue: markedOptions
};
