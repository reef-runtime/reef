//
// Reef data / schema definition file for job logs.
//

// Datasets can be listed, and uploaded on a separate page.
// When submitting a job, a dataset can be attached.
export interface Dateset {
    // Primary key, is unique. Always 64 characters long.
    id: string,
    // User-friendly name for this dataset.
    // If name is too long, use elipsis to hide the rest.
    name: string,
    // Size of the dataset in bytes. Display in a human-readable format, e.g. MB / KB.
    size: number,
}
