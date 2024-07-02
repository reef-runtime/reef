import { Separator } from '@/components/ui/separator';
import JobStatusIcon from '@/components/job-status';
import { IJob, IJobStatus } from '@/types/job';
import { Progress } from '@/components/ui/progress';
import classNames from 'classnames';

interface JobListItemProps {
  job: IJob;
}

const JobListItem: React.FC<JobListItemProps> = ({ job }) => {
  function handleClick() {
    window.location.href = '/jobs/detail/?id=' + encodeURIComponent(job.id);
  }

  return (
    <div className="w-full overflow-hidden">
      <ul
        key={job.id}
        className="space-y-2 p-2 rounded-xl transition-shadow duration-300 hover:shadow-lg cursor-pointer"
        onClick={handleClick}
      >
        <li className="text-sm text-muted-foreground font-bold flex items-center justify-between space-x-1">
          <div className="flex items-center">
            <JobStatusIcon job={job} />
            <span className="text-primary">{job.name}</span>
          </div>
          <span
            className={classNames('', {
              'text-muted-foreground': job.status !== IJobStatus.StatusRunning,
              'text-primary': job.status === IJobStatus.StatusRunning,
            })}
          >
            {(function () {
              switch (job.status) {
                case IJobStatus.StatusQueued:
                  return 'QUEUED';
                case IJobStatus.StatusStarting:
                  return 'STARTING';
                case IJobStatus.StatusRunning:
                  return `${Math.floor(job.progress * 100)}%`;
                case IJobStatus.StatusDone:
                  return job.result?.success ? 'SUCCESS' : 'FAILURE';
              }
            })()}
          </span>
        </li>

        <li>
          {(function () {
            if (job.status !== IJobStatus.StatusRunning) {
              return null;
            }
            const percentage = Math.floor(job.progress * 100);
            return (
              <Progress
                value={percentage}
                className="h-1.5 w-full bg-muted/90"
              />
            );
          })()}
        </li>

        <li className="text-xs font-medium leading-none overflow-hidden">
          <span className="text-nowrap text-muted-foreground">{job.id}</span>
        </li>
      </ul>
      <Separator />
    </div>
  );
};

export default JobListItem;
