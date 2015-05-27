#lang scheme/base

(require scribble/doclang
         (except-in scribble/manual defthing defthing* codeblock)
         scribble/html-properties
         scribble/private/manual-defaults

         concourse/docs/defthing
         concourse/docs/literate
         concourse/docs/pipeline-image
         concourse/docs/codeblock)

(provide (except-out (all-from-out scribble/doclang) #%module-begin)
         (all-from-out scribble/manual)
         (rename-out [module-begin #%module-begin])
         manual-doc-style
         (all-from-out concourse/docs/defthing)
         (all-from-out concourse/docs/literate)
         (all-from-out concourse/docs/pipeline-image)
         (all-from-out concourse/docs/codeblock))

(define-syntax-rule (module-begin id . body)
  (#%module-begin id post-process () . body))
