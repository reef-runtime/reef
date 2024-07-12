'use client';

import { useEffect, useState } from 'react';
import Split from 'react-split-grid';
import { useTheme } from 'next-themes';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';

import JobStatusIcon from '@/components/job-status';
import { BookOpenText } from 'lucide-react';
import IconRustLanguage from '@/components/rust-logo';
import IconCLanguage from '@/components/c-logo';

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Separator } from '@/components/ui/separator';
import { Button } from '@/components/ui/button';
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectGroup,
  SelectItem,
} from '@/components/ui/select';
import {
  Form,
  FormField,
  FormItem,
  FormLabel,
  FormControl,
  FormMessage,
} from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { useToast } from '@/components/ui/use-toast';
import { Label } from '@/components/ui/label';

import CodeMirror from '@uiw/react-codemirror';
import { rust } from '@codemirror/lang-rust';
import { cpp } from '@codemirror/lang-cpp';
import { vscodeDark, vscodeLight } from '@uiw/codemirror-theme-vscode';

import { JobLanguage, ITemplate } from '@/types/template';
import { useTemplates } from '@/stores/templates.store';
import { useDatasets } from '@/stores/datasets.store';

import { GetSocket, topicSingleJob } from '@/lib/websocket';
import { displayJobStatus, IJob, IJobStatus } from '@/types/job';
import { Progress } from '@/components/ui/progress';
import { displayLogKind, ILogKind } from '@/types/log';
import JobOutput from '@/components/job-output';
import JobProgress from '@/components/job-progress';

const SOURCE_CODE_KEY = 'source_code';
const TEMPLATE_KEY = 'template_id';

interface CompileRes {
  success: boolean;
  message: string;
  error: string;
}

interface SubmitRes {
  id: string;
}

const schema = z.object({
  name: z.string().min(2).max(50),
  language: z.enum(['c', 'rust']),
  sourceCode: z.string(),
  datasetId: z.string().optional(),
  datasetFile: z.any().optional(),
});

export default function Page() {
  const { toast } = useToast();
  const [response, setResponse] = useState<CompileRes>({
    success: false,
    message: '',
    error: '',
  });

  const { datasets, fetchDatasets, uploadDataset } = useDatasets();

  const { templates, setTemplates, fetchTemplates } = useTemplates();
  const [template, setTemplateInternal] = useState<ITemplate>({
    id: '',
    name: '',
    language: 'c',
    code: '',
    dataset: '',
  });
  const [templateFresh, setTemplateFresh] = useState<boolean>(true);

  function setTemplate(tmpl: ITemplate) {
    localStorage.setItem(TEMPLATE_KEY, tmpl.id);
    setTemplateInternal(tmpl);
  }

  function loadTemplate() {
    localStorage.removeItem(SOURCE_CODE_KEY);
    setTemplateFresh(true);
  }

  // Load datasets and templates on load.
  /* eslint-disable react-hooks/exhaustive-deps */
  useEffect(() => {
    fetchDatasets();
    fetchTemplates();
  }, []);
  /* eslint-enable react-hooks/exhaustive-deps */

  useEffect(() => {
    if (templates.length === 0) {
      return;
    }

    setTemplateFresh(true);

    const tmpl = loadTemplateFromStorage();
    if (!tmpl) setTemplate(templates[0]);
  }, [templates]);

  const form = useForm<z.infer<typeof schema>>({
    resolver: zodResolver(schema),
    defaultValues: {
      name: '',
      language: 'c',
      sourceCode: '',
    },
  });

  const onSubmit = async (values: z.infer<typeof schema>) => {
    console.log(values);

    let datasetId = values.datasetId;
    if (
      values.datasetFile &&
      values.datasetFile instanceof FileList &&
      values.datasetFile.length > 0
    ) {
      const newDataset = await uploadDataset(values.datasetFile[0]);
      form.setValue('datasetId', newDataset.id);
    }

    if (!datasetId) {
      toast({
        title: 'Error',
        description: 'No dataset selected',
      });
      return;
    }

    const res = await fetch('/api/jobs/submit', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(values),
    });

    if (res.status != 200) {
      const compileRes = (await res.json()) as CompileRes;
      setResponse(compileRes);

      toast({
        title: 'Error',
        description: 'Check console for more information',
      });
    }

    const submitRes = (await res.json()) as SubmitRes;
    setJobId(submitRes.id);
  };

  const { theme } = useTheme();

  let [columns, setColumns] = useState(`2fr 16px 1fr`);

  const handleDrag = (_direction: any, _track: any, gridTemplateStyle: any) => {
    setColumns(gridTemplateStyle);
  };

  useEffect(() => {
    const loadedCode = loadSourceCode();

    if (!templateFresh) {
      console.log('template not fresh');
      return;
    }

    if (!template) {
      console.log('illegal template selection');
      return;
    }

    let usedTemplate = template;

    const loadedTempl = loadTemplateFromStorage();
    if (loadedTempl) {
      const searched = templates.find((t) => t.id === loadedTempl);
      if (searched) usedTemplate = searched;
    }

    console.log('USED EFFECT');
    setLanguage(usedTemplate.language);
    form.setValue('language', usedTemplate.language);

    setDataset(usedTemplate.dataset);
    form.setValue('datasetId', usedTemplate.dataset);

    if (loadedCode) {
      setSourceCodeInternal(loadedCode);
      form.setValue('sourceCode', loadedCode);
    } else {
      setSourceCode(usedTemplate.code);
      form.setValue('sourceCode', usedTemplate.code);
    }

    form.setValue('name', usedTemplate.name);

    setTemplateFresh(false);
  }, [template, templateFresh, form]);

  const [language, setLanguage] = useState<JobLanguage>(template.language);
  const [dataset, setDataset] = useState<string>(template.dataset);
  const [sourceCode, setSourceCodeInternal] = useState<string>(template.code);

  function setSourceCode(newCode: string) {
    console.log('SET', newCode);
    localStorage.setItem(SOURCE_CODE_KEY, newCode);
    setSourceCodeInternal(newCode);
  }

  function loadTemplateFromStorage(): string | null {
    const tmlp = localStorage.getItem(TEMPLATE_KEY);
    return tmlp;
  }

  function loadSourceCode(): string | null {
    const code = localStorage.getItem(SOURCE_CODE_KEY);
    return code;
  }

  // File Size handling
  const UNITS = ['byte', 'kilobyte', 'megabyte', 'gigabyte'];
  const BYTES_PER_KB = 1000;

  function humanFileSize(sizeBytes: number | bigint): string {
    let size = Math.abs(Number(sizeBytes));

    let u = 0;
    while (size >= BYTES_PER_KB && u < UNITS.length - 1) {
      size /= BYTES_PER_KB;
      ++u;
    }

    return new Intl.NumberFormat([], {
      style: 'unit',
      unit: UNITS[u],
      unitDisplay: 'short',
      maximumFractionDigits: 1,
    }).format(size);
  }

  const [jobId, setJobId] = useState<string | null>(null);

  const [job, setJob] = useState<IJob | null>(null);

  useEffect(() => {
    if (!jobId) {
      return;
    }

    const sock = GetSocket();
    sock.unsubscribeAll();

    sock.subscribe(topicSingleJob(jobId), (res) => {
      console.dir(res.data);
      setJob(res.data);
    });
  }, [jobId]);

  return (
    <Form {...form}>
      <form
        onSubmit={form.handleSubmit(onSubmit)}
        className="h-full p-4 space-y-4 xl:space-y-0 xl:space-x-4"
      >
        <Split
          gridTemplateColumns={columns}
          onDrag={handleDrag}
          cursor="col-resize"
          minSize={100}
          // @ts-ignore
          render={({ getGridProps, getGutterProps }) => (
            <div className="h-full grid split-grid" {...getGridProps()}>
              <Card className="split-column h-full w-full overflow-auto rounded-lg border bg-card text-card-foreground shadow-sm flex flex-col">
                <CardHeader className="w-full flex flex-row justify-between p-3 space-y-0 bg-background">
                  <div className="flex flex-row items-center gap-2">
                    <FormItem>
                      <FormControl>
                        <Select
                          onValueChange={(newTemplateID) => {
                            console.dir(newTemplateID);
                            const newT = templates.find(
                              (t) => t.id == newTemplateID
                            );
                            if (!newT) {
                              throw `Illegal item: ${newT}`;
                            }
                            setTemplate(newT);
                          }}
                          value={template.id}
                        >
                          <SelectTrigger className="w-[20rem]">
                            <SelectValue placeholder="Select a Template" />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectGroup>
                              {templates.map((t, _) => {
                                return (
                                  <SelectItem value={t.id} key={t.id}>
                                    {t.name}
                                  </SelectItem>
                                );
                              })}
                            </SelectGroup>
                          </SelectContent>
                        </Select>
                      </FormControl>
                      <FormMessage />
                    </FormItem>

                    <Button
                      onClick={loadTemplate}
                      type="button"
                      variant="outline"
                    >
                      Load Template
                    </Button>
                    <Button
                      onClick={() => {
                        console.log('cool docs');
                      }}
                      type="button"
                      variant="outline"
                      className="text-center"
                    >
                      <BookOpenText
                        strokeWidth={1.5}
                        size={20}
                        className="mr-2 mt-[1px]"
                      />
                      Docs
                    </Button>
                  </div>

                  <div className="flex flex-row items-center gap-2">
                    <span className="text-4xl text-muted-foreground self-center mx-[1rem]">
                      {language === 'c' ? (
                        <IconCLanguage />
                      ) : (
                        <IconRustLanguage />
                      )}
                    </span>

                    <ul className="w-36">
                      <li className="flex justify-between w-full">
                        <span className="text-sm text-muted-foreground">
                          Lines Of Code
                        </span>
                        <span>{sourceCode.split('\n').length}</span>
                      </li>
                      <li className="flex justify-between w-full">
                        <span className="text-sm text-muted-foreground">
                          Dataset Size
                        </span>
                        {(function () {
                          const sz = datasets.find(
                            (d) => d.id === form.getValues('datasetId')
                          )?.size;

                          if (!sz) {
                            return (
                              <span className="text-sm text-muted-foreground">
                                N/A
                              </span>
                            );
                          }

                          return (
                            <span className="text-sm">{humanFileSize(sz)}</span>
                          );
                        })()}
                      </li>
                    </ul>
                  </div>
                </CardHeader>
                <Separator></Separator>
                <CardContent className="p-0">
                  <FormField
                    control={form.control}
                    name="sourceCode"
                    render={({ field }) => (
                      <FormItem style={{ height: '100%' }}>
                        <FormControl>
                          <CodeMirror
                            {...field}
                            style={{ height: '100%' }}
                            className={'codeEditor'}
                            value={sourceCode}
                            lang={language}
                            height="100%"
                            theme={theme === 'dark' ? vscodeDark : vscodeLight}
                            extensions={language === 'c' ? [cpp()] : [rust()]}
                            basicSetup={{ tabSize: language === 'c' ? 2 : 4 }}
                            onChange={(value, _) => {
                              setSourceCode(value);
                              field.onChange(value);
                            }}
                          />
                        </FormControl>
                      </FormItem>
                    )}
                  />
                </CardContent>
              </Card>

              <div
                {...getGutterProps('column', 1)}
                className="gutter gutter-vertical"
              ></div>

              <div className="split-column h-full overflow-auto flex flex-col space-y-4">
                <Card className="w-full flex flex-col bg-background">
                  <CardHeader className="px-3 pb-0">
                    <CardTitle>Job Submission</CardTitle>
                  </CardHeader>
                  <CardContent className="flex flex-col p-3">
                    <div className="flex justify-stretch flex-wrap gap-x-2">
                      <FormField
                        control={form.control}
                        name="name"
                        render={({ field }) => (
                          <FormItem className="grow mt-2">
                            <FormLabel>Job Name</FormLabel>
                            <FormControl>
                              <Input placeholder="Job Name" {...field} />
                            </FormControl>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                      <FormField
                        control={form.control}
                        name="language"
                        render={({ field }) => (
                          <FormItem className="w-full xl:w-[30%] mt-2">
                            <FormLabel>Language</FormLabel>
                            <FormControl>
                              <Select
                                onValueChange={(newLang) => {
                                  field.onChange(newLang);
                                  setLanguage(newLang as JobLanguage);
                                }}
                                value={language}
                              >
                                <SelectTrigger className="w-full">
                                  <SelectValue placeholder="Select a language" />
                                </SelectTrigger>
                                <SelectContent>
                                  <SelectGroup>
                                    <SelectItem value="c">C</SelectItem>
                                    <SelectItem value="rust">Rust</SelectItem>
                                  </SelectGroup>
                                </SelectContent>
                              </Select>
                            </FormControl>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                    </div>
                    <div className="flex justify-stretch flex-wrap gap-x-2">
                      <FormField
                        control={form.control}
                        name="datasetId"
                        render={({ field }) => (
                          <FormItem className="grow mt-2">
                            <FormLabel>Select Dataset</FormLabel>
                            <FormControl>
                              <Select
                                onValueChange={(newDataset) => {
                                  console.dir(newDataset);
                                  field.onChange(newDataset);
                                  setDataset(newDataset);
                                }}
                                value={dataset}
                              >
                                <SelectTrigger className="w-full">
                                  <SelectValue placeholder="Select an Existing Dataset" />
                                </SelectTrigger>
                                <SelectContent>
                                  <SelectGroup>
                                    {datasets.map((dataset, i) => (
                                      <SelectItem value={dataset.id} key={i}>
                                        {dataset.name}
                                      </SelectItem>
                                    ))}
                                  </SelectGroup>
                                </SelectContent>
                              </Select>
                            </FormControl>
                            <FormMessage />
                          </FormItem>
                        )}
                      />

                      <FormItem className="w-full xl:w-[30%] mt-2">
                        <FormLabel className="w-full inline-block text-nowrap text-ellipsis overflow-hidden">
                          Upload New Dataset
                        </FormLabel>
                        <FormControl>
                          <Input
                            className=""
                            id="datasetFile"
                            type="file"
                            onChange={(e) => {
                              if (
                                !e.target.files ||
                                e.target.files.length === 0
                              ) {
                                toast({
                                  title: 'Not Uploaded',
                                  description:
                                    'No file was selected for dataset upload',
                                });
                                return;
                              }

                              const fileCnt = e.target.files.length;

                              for (let i = 0; i < fileCnt; i++) {
                                const file = e.target.files[i];
                                console.log(file);

                                uploadDataset(file).then((newDataset) => {
                                  form.setValue('datasetId', newDataset.id);
                                  toast({
                                    title: 'File Uploaded Successfully',
                                    description: `Created new dataset '${newDataset.id.substring(0, 16)}...'`,
                                  });
                                });
                              }
                            }}
                          />
                        </FormControl>
                      </FormItem>
                    </div>
                  </CardContent>

                  <div className="grow" />
                  <Button type="submit" className="rounded-t-none">
                    Submit Job
                  </Button>
                </Card>

                <Card className="h-full w-full flex flex-col bg-background">
                  <div
                    className="h-full w-full px-4 py-3 overflow-auto"
                    style={{
                      fontFamily: 'monospace',
                      fontSize: '0.9rem',
                      boxSizing: 'border-box',
                    }}
                  >
                    <div className="flex flex-wrap justify-between gap-30">
                      <span className="font-bold whitespace-nowrap">
                        {response.message
                          ? response.message.toUpperCase()
                          : 'JOB STATUS'}
                      </span>
                      {job ? (
                        <a
                          className="text-ellipsis overflow-hidden text-nowrap text-xs underline"
                          href={`/jobs/detail/?id=${job?.id}`}
                        >
                          ID: {job.id}
                        </a>
                      ) : null}
                    </div>

                    <Separator className="my-5"></Separator>

                    {(function () {
                      if (response.message && response.message != '') {
                        return (
                          <div
                            dangerouslySetInnerHTML={{
                              __html: response.error
                                .replaceAll('\n', '<br>')
                                .replaceAll(' ', '&nbsp;'),
                            }}
                          ></div>
                        );
                      } else {
                        return (
                          <div>
                            <div className="h-[3rem] flex flex-col justify-center gap-3 mb-3">
                              <div className="flex justify-between align-center w-full">
                                <JobStatusIcon job={job}></JobStatusIcon>
                                {displayJobStatus(job)}
                              </div>
                              <JobProgress job={job}></JobProgress>
                            </div>

                            <JobOutput job={job} compact={true}></JobOutput>
                          </div>
                        );
                      }
                    })()}
                  </div>
                </Card>
              </div>
            </div>
          )}
        />
      </form>
    </Form>
  );
}
