#lang scheme/base

(require scribble/doclang 
         (except-in scribble/manual defthing defthing*)
         scribble/html-properties
         scribble/private/manual-defaults
         
         concourse/docs/defthing)

(provide (except-out (all-from-out scribble/doclang) #%module-begin)
         (all-from-out scribble/manual)
         (rename-out [module-begin #%module-begin])
         defthing
         manual-doc-style)

(define-syntax-rule (module-begin id . body)
  (#%module-begin id post-process () . body))
