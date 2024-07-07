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
  function handleClick() {
    if (!job) {
      return;
    }
    window.location.href = '/jobs/detail/?id=' + encodeURIComponent(job.id);
  }

  const percentage = job ? Math.floor(job.progress * 100) : 0;

  return (
    <div className="w-full overflow-hidden" onClick={handleClick}>
      <ul
        key={workerIndex}
        className={classNames(
          'space-y-2 p-2 rounded-xl transition-shadow duration-300 hover:shadow-lg cursor-pointer h-12',
          {
            'flex items-center': percentage < 1,
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
            {percentage < 1 ? null : (
              <span className="text-sm font-medium leading-none">
                {percentage} %
              </span>
            )}
          </span>
        </li>

        {percentage < 1 ? null : (
          <Progress value={percentage} className="h-1.5 w-full bg-muted/90" />
        )}
      </ul>
    </div>
  );
};

export default WorkerListItem;
