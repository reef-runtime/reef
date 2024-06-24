import { Separator } from '@/components/ui/separator';
import JobStatusIcon from '@/components/job-status';
import { IJob } from '@/types/job';

interface JobListItemProps {
  job: IJob;
}

const JobListItem: React.FC<JobListItemProps> = ({ job }) => {
  function handleClick() {
    window.location.href = '/jobs/detail/?id=' + encodeURIComponent(job.id);
  }

  return (
    <div>
      <Separator className="" />
      <ul
        key={job.id}
        className="space-y-2 p-2 rounded-xl transition-shadow duration-300 hover:shadow-lg cursor-pointer"
        onClick={handleClick}
      >
        <li className="text-sm text-muted-foreground font-bold flex items-center space-x-1">
          <JobStatusIcon job={job} />
          <span>{job.name}</span>
        </li>
        <li className="text-xs font-medium leading-none overflow-hidden">
          <span className="text-nowrap">{job.id}</span>
        </li>
      </ul>
      <Separator className="" />
    </div>
  );
};

export default JobListItem;
