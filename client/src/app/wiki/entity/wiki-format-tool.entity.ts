import { WikiFormat } from '../constants/wiki-format.enum';

export interface WikiFormatTool {
    format: WikiFormat;
    icon: string;
    label: string;
}
