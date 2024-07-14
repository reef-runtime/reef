import { Separator } from '@/components/ui/separator';
import JobStatusIcon from '@/components/job-status';
import { displayJobStatus, IJob, IJobStatus } from '@/types/job';
import classNames from 'classnames';
import JobProgress from './job-progress';

interface JobListItemProps {
  job: IJob;
}

const JobListItem: React.FC<JobListItemProps> = ({ job }) => {
  function handleClick() {
    window.location.href = '/jobs/detail/?id=' + encodeURIComponent(job.id);
  }

  return (
    <div className="w-full overflow-hidden" onClick={handleClick}>
      <ul
        key={job.id}
        className="space-y-2 p-2 rounded-xl transition-shadow duration-300 hover:shadow-lg cursor-pointer"
      >
        <li className="text-sm text-muted-foreground font-bold flex items-center justify-between space-x-1">
          <div className="flex items-center">
            <JobStatusIcon job={job} />
            <span className="ml-1 text-primary">{job.name}</span>
          </div>
          <span
            className={classNames('', {
              'text-muted-foreground': job.status !== IJobStatus.StatusRunning,
              'text-primary': job.status === IJobStatus.StatusRunning,
            })}
          >
            {displayJobStatus(job)}
          </span>
        </li>

        <li>
          <JobProgress job={job}></JobProgress>
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

export const JobListItemPlaceholder = () => {
  return (
    <div className="h-full flex flex-col place-items-center justify-center min-w-[300px]">
      <div className="text-xl text-muted-foreground">No Jobs to show</div>
    </div>
  );
};
