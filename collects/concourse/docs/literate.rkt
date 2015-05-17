#lang scheme/base

(require (only-in scribble/core style itemization nested-flow
                  compound-paragraph)
         (only-in racket/list split-at-right)
         (only-in scribble/html-properties css-addition))

(provide literate-segment literate)

(define (literate . segments)
  (itemization
    (style "literate" (list (css-addition "concourse.css")))
    segments))

(define (literate-segment . blocks)
  (define-values (paras codes) (split-at-right blocks 1))

  (if (null? paras)
    (list
      (nested-flow (style "para" null) codes)
      (nested-flow (style "code" null) '()))
    (list
      (nested-flow (style "para" null) paras)
      (nested-flow (style "code" null) codes))))
