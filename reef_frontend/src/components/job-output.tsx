import { FC } from 'react';
import { CogIcon, BanIcon } from 'lucide-react';
import classNames from 'classnames';
import { IJob, IJobStatus } from '@/types/job';
import { displayLogKind } from '@/types/log';
import { useTheme } from 'next-themes';

interface JobOutputProps {
  job?: IJob | null;
  compact: boolean;
}

const JobOutput: FC<JobOutputProps> = ({ job, compact }) => {
  if (!job || !job.logs) {
    return <span className="text-muted-foreground text-sm">No output</span>;
  }

  const { theme } = useTheme();
  const darkTheme = theme === 'dark';

  return job.logs
    ?.sort(
      (a, b) => new Date(a.created).getTime() - new Date(b.created).getTime()
    )
    .map((log, index) => {
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
          className={classNames('font-mono text-xs', {
            'text-white': darkTheme,
            'text-black': !darkTheme,
          })}
        >
          <span
            className={classNames({
              'text-green-500': darkTheme,
              'text-emerald-600': !darkTheme,
            })}
          >
            {prefix}
            {hour}:{minute}:{second}
          </span>
          <span
            className={classNames({
              'text-blue-300': darkTheme,
              'text-blue-800': !darkTheme,
            })}
          >
            {' '}
            [{displayLogKind(log.kind)}]
          </span>
          <span> {log.content}</span>
        </div>
      );
    });
};

export default JobOutput;
