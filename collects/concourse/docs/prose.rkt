#lang scheme/base

(require racket/runtime-path
         (only-in scribble/core style)
         (only-in scribble/html-properties css-addition))

(provide strike break-word)

(define-runtime-path prose.css "prose.css")

(define strike
  (style "strike"
         (list (css-addition (path->string prose.css)))))

(define break-word
  (style "break-word"
         (list (css-addition (path->string prose.css)))))

