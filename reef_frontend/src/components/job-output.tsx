import { FC } from 'react';

import { IJob } from '@/types/job';
import { displayLogKind } from '@/types/log';
import { ScrollArea } from './ui/scroll-area';

interface JobOutputProps {
  job?: IJob | null;
  compact: boolean;
}

const JobOutput: FC<JobOutputProps> = ({ job, compact }) => {
  return !job || (job.logs?.length ?? 0) === 0 ? (
    <div className="h-full flex flex-col justify-center items-center">
      <span className="text-muted-foreground text-sm text-nowrap max-w-min">
        No output
      </span>
    </div>
  ) : (
    <ScrollArea>
      {job.logs.map((log, index) => {
        const date = new Date(log.created);

        const day = String(date.getDay()).padStart(2, '0');
        const month = String(date.getMonth() + 1).padStart(2, '0');

        const hour = String(date.getHours()).padStart(2, '0');
        const minute = String(date.getMinutes()).padStart(2, '0');
        const second = String(date.getSeconds()).padStart(2, '0');

        let prefix = '';
        if (!compact) {
          prefix = `${month}/${day} `;
        }

        return (
          <div
            key={index}
            className="font-mono text-xs text-black dark:text-white"
          >
            <span className="text-emerald-600 dark:text-green-500">
              {prefix}
              {hour}:{minute}:{second}
            </span>
            <span className="text-blue-800 dark:text-blue-300">
              {' '}
              [{displayLogKind(log.kind)}]
            </span>
            <span> {log.content}</span>
          </div>
        );
      })}
    </ScrollArea>
  );
};

export default JobOutput;
