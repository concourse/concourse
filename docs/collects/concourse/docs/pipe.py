# Copyright (c) 2013-2014, Greg Hendershott.
# All rights reserved.
#
# Redistribution and use in source and binary forms, with or without
# modification, are permitted provided that the following conditions are
# met:
#
# - Redistributions of source code must retain the above copyright
#   notice, this list of conditions and the following disclaimer.
#
# - Redistributions in binary form must reproduce the above copyright
#   notice, this list of conditions and the following disclaimer in the
#   documentation and/or other materials provided with the distribution.
#
# THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
# "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
# LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
# A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
# HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
# SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
# LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
# DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
# THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
# (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
# OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

# This allows us to launch Python and pygments once, and pipe to it
# continuously. Input format is:
#
#     <lexer-name>
#     __END__
#     <code>
#     ...
#
#  OR
#
#     __EXIT__
#
# Output format is:
#
#     <html>
#     ...
#     __END__

import sys
import optparse
from pygments import highlight
from pygments.lexers import get_lexer_by_name
from pygments.util import ClassNotFound
from pygments.formatters import HtmlFormatter

formatter = HtmlFormatter(nowrap=True, encoding="utf-8")

lexer = ""
code = ""
py_version = sys.version_info.major
sys.stdout.write("ready\n")
sys.stdout.flush
while 1:
    line_raw = sys.stdin.readline()
    if not line_raw:
        break
    # Without trailing space, \n, or \n
    line = line_raw.rstrip()
    if line == '__EXIT__':
        break
    elif line == '__END__':
        # Lex input finished. Lex it.
        if py_version >= 3:
          sys.stdout.write(highlight(code, lexer, formatter).decode("utf-8"))
        else:
          sys.stdout.write(highlight(code, lexer, formatter))
        sys.stdout.write('\n__END__\n')
        sys.stdout.flush
        lexer = ""
        code = ""
    elif lexer == "":
        # Starting another lex. First line is the lexer name.
        try:
            lexer = get_lexer_by_name(line, encoding="guess")
        except ClassNotFound:
            lexer = get_lexer_by_name("text", encoding="guess")
    else:
        # Accumulate more code
        # Use `line_raw`: Do want trailing space, \n, \r
        code += line_raw

exit(0)
