import { Separator } from '@/components/ui/separator';
import JobStatusIcon from '@/components/job-status';
import { IJob } from '@/types/job';
import classNames from 'classnames';
import { Progress } from '@/components/ui/progress';

interface WorkerListItemProps {
  workerIndex: number;
  job?: IJob;
}

const WorkerListItem: React.FC<WorkerListItemProps> = ({
  job,
  workerIndex,
}) => {
  return (
    <div className="w-full overflow-hidden">
      <ul
        key={workerIndex}
        className={classNames(
          'space-y-2 p-2 rounded-xl transition-shadow duration-300 hover:shadow-lg cursor-pointer h-12',
          {
            'flex items-center': job === undefined,
          }
        )}
      >
        <li className="text-sm text-muted-foreground font-bold flex items-center space-x-1">
          <JobStatusIcon job={job} />
          <span
            className={classNames('text-sm font-medium leading-none', {
              'text-sm text-muted-foreground': job === undefined,
            })}
          >
            {job ? job.name : 'Worker Idle'}
          </span>

          <span className="text-sm font-medium leading-none">
            {(function () {
              if (!job) {
                return null;
              }

              const percentage = Math.floor(job.progress * 100);
              return (
                <span className="text-sm font-medium leading-none">
                  {percentage} %
                </span>
              );
            })()}
          </span>
        </li>

        {(function () {
          if (!job) {
            return null;
          }

          const percentage = Math.floor(job.progress * 100);
          return (
            <Progress value={percentage} className="h-1.5 w-full bg-muted/90" />
          );
        })()}
      </ul>
      <Separator className="" />
    </div>
  );
};

export default WorkerListItem;
