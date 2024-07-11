import { FC } from 'react';
import { CogIcon, BanIcon } from 'lucide-react';
import classNames from 'classnames';
import { IJob, IJobStatus } from '@/types/job';
import { Progress } from './ui/progress';

interface JobProgressProps {
  job?: IJob | null;
}

const JobProgress: FC<JobProgressProps> = ({ job }) => {
  if (!job || job?.status !== IJobStatus.StatusRunning || job.progress < 0.01)
    return null;

  const percentage = Math.floor(job.progress * 100);

  return (
    <div>
      <Progress value={percentage} className="h-1.5 w-full bg-muted/90" />
    </div>
  );
};

export default JobProgress;
