import { FC } from 'react';
import { CogIcon, BanIcon } from 'lucide-react';
import classNames from 'classnames';
import { IJob, IJobStatus } from '@/types/job';

interface JobStatusIconProps {
  job?: IJob;
}

const JobStatusIcon: FC<JobStatusIconProps> = ({ job }) => {
  if (job?.status === IJobStatus.StatusStarting) {
    return (
      <div className="flex justify-center w-5">
        <span className="relative flex h-3 w-3">
          <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-sky-400 opacity-75"></span>
          <span className="relative inline-flex rounded-full h-3 w-3 bg-sky-500"></span>
        </span>
      </div>
    );
  }

  if (job?.status === IJobStatus.StatusRunning) {
    return (
      <div className="flex justify-center w-5">
        <CogIcon className="h-5 w-5 text-orange-400 animate-spin-slow" />
      </div>
    );
  }

  if (job === undefined) {
    return (
      <div className="flex justify-center w-5">
        <BanIcon className="h-4 w-4 text-gray-300" />
      </div>
    );
  }

  return (
    <div className="flex justify-center w-5">
      <span
        className={classNames('w-3 h-3 rounded-full', {
          'bg-gray-400': job?.status === IJobStatus.StatusQueued,
          'bg-green-500':
            job?.status === IJobStatus.StatusDone && job.result?.success,
          'bg-red-500 animate-ping':
            job?.status === IJobStatus.StatusDone && !job.result?.success,
        })}
      ></span>
    </div>
  );
};

export default JobStatusIcon;
