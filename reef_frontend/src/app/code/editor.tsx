import React, { FormEvent } from 'react';
import { rust } from '@codemirror/lang-rust';
// import { cpp } from '@codemirror/lang-cpp';
import { useEffect, useMemo, useRef } from 'react';
import CodeMirror from '@uiw/react-codemirror';
import { vscodeDark } from '@uiw/codemirror-theme-vscode';
import { VariantProps } from 'class-variance-authority';

const extensions = [rust()];

export interface EditorProps extends React.HTMLAttributes<HTMLInputElement> {
  code: string;
  className: string;
  onSourceChange: (sourceCode: string) => void;
}

export default function Editor({
  className,
  code,
  onSourceChange,
}: EditorProps) {
  const onChangeInternal = React.useCallback(
    (sourceCode: any, _viewUpdate: any) => {
      onSourceChange(sourceCode);
    },
    []
  );

  onSourceChange(code);

  return (
    <CodeMirror
      style={{ height: '100%' }}
      className={className}
      value={code}
      lang="c"
      height="100%"
      theme={vscodeDark}
      extensions={extensions}
      onChange={onChangeInternal}
    />
  );
}

export { Editor };
