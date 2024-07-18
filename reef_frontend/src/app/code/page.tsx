'use client';

import React, { useEffect, useState } from 'react';
import Split from 'react-split-grid';
import { useTheme } from 'next-themes';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';

import JobStatusIcon from '@/components/job-status';
import { BookOpenText } from 'lucide-react';
import IconRustLanguage from '@/components/rust-logo';
import IconCLanguage from '@/components/c-logo';

import { Separator } from '@/components/ui/separator';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {
  Form,
  FormField,
  FormItem,
  FormLabel,
  FormControl,
  FormMessage,
} from '@/components/ui/form';
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectGroup,
  SelectItem,
} from '@/components/ui/select';
import { Input } from '@/components/ui/input';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { useToast } from '@/components/ui/use-toast';

import CodeMirror from '@uiw/react-codemirror';
import { rust } from '@codemirror/lang-rust';
import { cpp } from '@codemirror/lang-cpp';
import { vscodeDark, vscodeLight } from '@uiw/codemirror-theme-vscode';

import { JobLanguage } from '@/types/template';
import { useTemplates } from '@/stores/templates.store';
import { useDatasets } from '@/stores/datasets.store';

import { GetSocket, topicSingleJob } from '@/lib/websocket';
import { displayJobStatus, IJob } from '@/types/job';
import JobOutput from '@/components/job-output';
import JobProgress from '@/components/job-progress';
import DocsPage from '@/components/docs-page';
import { cStdDoc } from '@/lib/reef_std_doc/c';
import { rustStdDoc } from '@/lib/reef_std_doc/rust';
import { humanFileSize } from '@/lib/utils';

import { useReefSession } from '@/stores/session.store';

const EDITOR_STATE_KEY = 'editor_state';

const RATE_LIMITING_WAIT_MS = 5000;

interface CompileRes {
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
  const { theme } = useTheme();

  let [columns, setColumns] = useState(`2fr 16px 1fr`);
  const handleDrag = (_direction: any, _track: any, gridTemplateStyle: any) => {
    setColumns(gridTemplateStyle);
  };

  const { datasets, fetchDatasets, uploadDataset } = useDatasets();
  const { templates, fetchTemplates } = useTemplates();

  // Load datasets and templates on load.
  /* eslint-disable react-hooks/exhaustive-deps */
  useEffect(() => {
    fetchDatasets();
    fetchTemplates();
  }, []);
  /* eslint-enable react-hooks/exhaustive-deps */

  const [selectedTemplateId, setSelectedTemplateId] = useState<string>('');

  const [sourceCode, setSourceCodeState] = useState<string>('');
  const [language, setLanguage] = useState<JobLanguage>('c');
  const [datasetId, setDatasetId] = useState<string>('');

  function saveEditorState() {
    localStorage.setItem(
      EDITOR_STATE_KEY,
      JSON.stringify({
        sourceCode: form.getValues().sourceCode,
        name: form.getValues().name,
        language: form.getValues().language,
        datasetId: form.getValues().datasetId,
      })
    );
  }

  function setSourceCode(newCode: string) {
    form.setValue('sourceCode', newCode);
    saveEditorState();
    setSourceCodeState(newCode);
  }

  function setEditorState(
    sourceCode: string,
    name: string,
    language: 'c' | 'rust',
    datasetId: string
  ) {
    setSourceCodeState(sourceCode);
    form.setValue('sourceCode', sourceCode);

    form.setValue('name', name);

    setLanguage(language);
    form.setValue('language', language);

    setDatasetId(datasetId);
    form.setValue('datasetId', datasetId);
  }

  // if the template set after initial page load is completed
  const [templateLoaded, setTemplateLoaded] = useState<boolean>(false);

  // wait for templates and datasets to load
  useEffect(() => {
    if (templates.length === 0 || datasets.length === 0 || templateLoaded) {
      return;
    }
    setTemplateLoaded(true);

    // try to load previous state from localStorage
    let state = localStorage.getItem(EDITOR_STATE_KEY);
    if (state) {
      let editorState = JSON.parse(state);
      setEditorState(
        editorState.sourceCode,
        editorState.name,
        editorState.language,
        editorState.datasetId
      );
    } else {
      let defaultTemplate = templates[0];
      setEditorState(
        defaultTemplate.code,
        defaultTemplate.name,
        defaultTemplate.language,
        defaultTemplate.dataset
      );
    }
  }, [templates, datasets, templateLoaded]);

  function loadSelectedTemplate() {
    let template = templates.find((t) => t.id === selectedTemplateId);
    if (template) {
      setEditorState(
        template.code,
        template.name,
        template.language,
        template.dataset
      );
    }

    saveEditorState();
  }

  const form = useForm<z.infer<typeof schema>>({
    resolver: zodResolver(schema),
    defaultValues: {
      name: '',
      language: 'c',
      sourceCode: '',
    },
  });

  const [canSubmit, setCanSubmit] = useState<boolean>(true);
  const [compileError, setCompileError] = useState<CompileRes | null>(null);

  const { session, fetchSession } = useReefSession();
  useEffect(() => {
    fetchSession(null);
  }, []);

  const onSubmit = async (values: z.infer<typeof schema>) => {
    if (!values.datasetId) {
      form.setError('datasetId', { message: 'No Dataset selected' });
      return;
    }

    setJobId(null);
    setCompileError({
      error: 'compiling...',
      message: '',
    });

    if (!session.isAdmin) {
      setCanSubmit(false);
      setTimeout(() => {
        setCanSubmit(true);
      }, RATE_LIMITING_WAIT_MS);
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
      setCompileError(compileRes);

      toast({
        title: 'Error',
        description: 'Check console for more information',
      });
      return;
    }

    const submitRes = (await res.json()) as SubmitRes;
    setCompileError(null);
    setJobId(submitRes.id);
  };

  const [jobId, setJobId] = useState<string | null>(null);
  const [job, setJob] = useState<IJob | null>(null);

  useEffect(() => {
    const sock = GetSocket();
    sock.unsubscribeAll();

    if (!jobId) {
      setJob(null);
      return;
    }

    sock.subscribe(topicSingleJob(jobId), (res) => {
      setJob(res.data);
    });
  }, [jobId]);

  const [isDocsDialogOpen, setIsDocsDialogOpen] = useState(false);

  useEffect(() => {
    // KEY BINDS.
    // CTRL + S  => Save current code.
    document.addEventListener('keydown', (e) => {
      if (e.ctrlKey && e.key === 's') {
        e.preventDefault();
        return;
      }
    });
  });

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
                          onValueChange={(newId) => {
                            setSelectedTemplateId(newId);
                          }}
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
                      onClick={loadSelectedTemplate}
                      type="button"
                      variant="outline"
                    >
                      Load Template
                    </Button>

                    <Button
                      onClick={() => {
                        setIsDocsDialogOpen(true);
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
                className="gutter gutter-vertical cursor-col-resize"
              ></div>

              <div className="split-column h-full flex flex-col space-y-4 overflow-hidden">
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
                              <Input
                                placeholder="Job Name"
                                {...field}
                                onChange={(e) => {
                                  field.onChange(e);
                                  saveEditorState();
                                }}
                              />
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
                                value={language}
                                onValueChange={(newLang) => {
                                  field.onChange(newLang);
                                  setLanguage(newLang as JobLanguage);
                                  saveEditorState();
                                }}
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
                                onValueChange={(newId) => {
                                  field.onChange(newId);
                                  setDatasetId(newId);
                                  saveEditorState();
                                }}
                                value={datasetId}
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

                                uploadDataset(file).then((newDataset) => {
                                  form.setValue('datasetId', newDataset.id);
                                  setDatasetId(newDataset.id);
                                  toast({
                                    title: 'File Uploaded Successfully',
                                    description: `Created new dataset '${newDataset.id.substring(0, 16)}...'`,
                                  });
                                  saveEditorState();
                                });
                              }
                            }}
                          />
                        </FormControl>
                      </FormItem>
                    </div>
                  </CardContent>

                  <Button
                    type="submit"
                    className="rounded-t-none"
                    disabled={!canSubmit}
                  >
                    Submit Job
                  </Button>
                </Card>

                <Card className="w-full grow flex flex-col bg-background px-4 py-3 font-mono text-[0.9rem] overflow-hidden">
                  <div className="flex flex-wrap justify-between gap-30">
                    <span className="font-bold whitespace-nowrap">
                      {compileError?.message
                        ? compileError.message.toUpperCase()
                        : 'JOB STATUS'}
                    </span>
                    {job ? (
                      <a
                        className="text-ellipsis overflow-hidden text-nowrap text-xs underline mb-1"
                        href={`/jobs/detail/?id=${job?.id}`}
                      >
                        ID: {job.id}
                      </a>
                    ) : null}
                  </div>

                  <Separator className="mt-2 mb-3"></Separator>

                  {compileError ? (
                    <div className="grow overflow-auto whitespace-pre">
                      {compileError.error}
                    </div>
                  ) : (
                    <React.Fragment>
                      <div className="flex flex-row justify-between align-center content-center w-full mb-2">
                        <JobStatusIcon job={job}></JobStatusIcon>
                        {displayJobStatus(job)}
                      </div>
                      <JobProgress job={job} className="mb-3"></JobProgress>

                      <JobOutput job={job} compact={true}></JobOutput>
                    </React.Fragment>
                  )}
                </Card>
              </div>
            </div>
          )}
        />
      </form>

      <Dialog open={isDocsDialogOpen} onOpenChange={setIsDocsDialogOpen}>
        <DialogContent className="h-[90svh] flex flex-col max-w-[1000px]">
          <DialogHeader>
            <DialogTitle>Reef Standard Library Documentation</DialogTitle>
            <DialogDescription>
              Here you can read about the various functions that you can use in
              the programs you submit to Reef.
            </DialogDescription>
          </DialogHeader>
          <Tabs defaultValue="c" className="grow overflow-hidden flex flex-col">
            <TabsList className="mb-2 max-w-min">
              <TabsTrigger value="c">C</TabsTrigger>
              <TabsTrigger value="rust">Rust</TabsTrigger>
            </TabsList>

            <ScrollArea>
              <TabsContent value="c">
                <DocsPage docs={cStdDoc} />
              </TabsContent>
              <TabsContent value="rust">
                <DocsPage docs={rustStdDoc} />
              </TabsContent>
            </ScrollArea>
          </Tabs>
        </DialogContent>
      </Dialog>
    </Form>
  );
}
