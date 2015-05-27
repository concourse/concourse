#lang scheme/base

(require racket/runtime-path
         scribble/core
         scribble/base
         (only-in scribble/html-properties install-resource))

(provide pipeline-image)

(define-runtime-path pipeline-svg.css "pipeline-svg.css")

(define (pipeline-image . image-args)
  (list
    (elem #:style (style "inst-css"
                         (list (install-resource (path->string pipeline-svg.css)))))

    (apply image image-args #:style "pipeline" #:suffixes '(".svg"))))
