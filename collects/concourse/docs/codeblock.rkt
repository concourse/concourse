#lang scheme/base

; Copyright (c) 2013-2014, Greg Hendershott.
; All rights reserved.
;
; Redistribution and use in source and binary forms, with or without
; modification, are permitted provided that the following conditions are
; met:
;
; - Redistributions of source code must retain the above copyright
;   notice, this list of conditions and the following disclaimer.
;
; - Redistributions in binary form must reproduce the above copyright
;   notice, this list of conditions and the following disclaimer in the
;   documentation and/or other materials provided with the distribution.
;
; THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
; "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
; LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
; A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
; HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
; SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
; LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
; DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
; THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
; (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
; OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

(require racket/match
         racket/runtime-path
         (only-in racket/port copy-port with-input-from-string)
         (only-in racket/system process)
         (only-in racket/string string-append* string-trim string-join)
         (only-in html read-html-as-xml)
         (only-in scribble/base verbatim)
         (only-in scribble/core element style nested-flow)
         (only-in scribble/html-properties css-addition)
         (only-in xml pcdata? pcdata-string
                      element? element-content element-attributes
                      entity? entity-text
                      attribute-name attribute-value))

(provide codeblock)

(define current-python-executable (make-parameter "python"))

;; Process that runs Python with our pipe.py script.

(define-values (pyg-in pyg-out pyg-pid pyg-err pyg-proc)
  (values #f #f #f #f #f))

(define-runtime-path pipe.py "pipe.py")

(define start
  (let ([start-attempted? #f])
    (lambda ()
      (unless start-attempted?
        (set! start-attempted? #t)
        (let ([python (find-executable-path (current-python-executable))])
          (if python
            (begin
              (match (process (format "~a -u ~a" python pipe.py))
                [(list in out pid err proc)
                 (set!-values (pyg-in pyg-out pyg-pid pyg-err pyg-proc)
                              (values in out pid err proc))
                 (file-stream-buffer-mode out 'line)
                 (match (read-line pyg-in 'any)  ;; consume "ready" line or EOF
                   [(? eof-object?) (say-no-pygments)]
                   [_ (say-pygments)])]
                [_ (say-no-pygments)]))
            (displayln "Python not found. Using plain `pre` blocks.")))))))

(define (say-pygments)
  (displayln "Using Pygments."))

(define (say-no-pygments)
  (displayln "Pygments not found."))

(define (running?)
  (and pyg-proc
       (eq? (pyg-proc 'status) 'running)))

(define (stop)
  (when (running?)
    (displayln "__EXIT__" pyg-out)
    (begin0 (or (pyg-proc 'exit-code) (pyg-proc 'kill))
      (close-input-port pyg-in)
      (close-output-port pyg-out)
      (close-input-port pyg-err)))

  (void))

(exit-handler
 (let ([old-exit-handler (exit-handler)])
   (lambda (v)
     (stop)
     (old-exit-handler v))))

(define (codeblock lang . code)
  (define (default code)
    (apply verbatim code))

  (unless (running?)
    (start))

  (cond
    [(running?)
      (displayln lang pyg-out)
      (displayln (string-append* code) pyg-out)
      (displayln "__END__" pyg-out)

      (let loop ([s ""])
        (match (read-line pyg-in 'any)
          ["__END__"
            (let ([stripped-code (string-trim s "\n" #:left? #f #:repeat? #t)])
              (render-spans-as-table
                (with-input-from-string stripped-code
                  read-html-as-xml)))]

          [(? string? v)
            (loop (string-append s v "\n"))]

          [_
            (copy-port pyg-err (current-output-port))
            (default code)]))]

    [else (default code)]))

(define (render-spans-as-table spans)
  (nested-flow (style "pygmentized" (list (css-addition "concourse.css")))
    (let ([rendered (map render-elem spans)])
      (list (apply verbatim rendered)))))

(define (render-elem e)
  (cond
    [(pcdata? e) (pcdata-string e)]
    [(element? e) (element (elem-style e) (map render-elem (element-content e)))]
    [(entity? e) (format "~a" (integer->char (entity-text e)))]
    [else (format "unknown element: ~v" e)]))

(define (elem-style e)
  (let ([class (element-class e)])
    (if (= 0 (string-length class))
      'tt
      (style class '(omitable)))))

(define (element-class e)
  (string-append* (map extract-class-value (element-attributes e))))

(define (extract-class-value a)
  (if (eq? (attribute-name a) 'class)
    (string-append "stt " (attribute-value a)) ; TODO: super janky class joining
    ""))
