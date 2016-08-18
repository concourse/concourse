port module LoginPage exposing (main)

import Dict
import Erl
import Http
import Navigation
import String
import UrlParser exposing ((</>), s)

import Login exposing (Page (..), PageWithRedirect)

main : Program Never
main =
  Navigation.program
    (Navigation.makeParser pathnameParser)
    { init = Login.init
    , update = Login.update
    , urlUpdate = Login.urlUpdate
    , view = Login.view
    , subscriptions = always Sub.none
    }

pathnameParser : Navigation.Location -> Result String PageWithRedirect
pathnameParser location =
  let
    redirect =
      Http.uriDecode <|
        Maybe.withDefault "" <|
          Dict.get "redirect" (Erl.parse location.search).query
  in
    UrlParser.parse
      (redirectInserter redirect)
      pageParser
      (String.dropLeft 1 location.pathname)

redirectInserter : String -> Page -> PageWithRedirect
redirectInserter uri page =
  {page = page, redirect = uri}

pageParser : UrlParser.Parser (Page -> a) a
pageParser =
  UrlParser.oneOf
    [ UrlParser.format TeamSelectionPage (s "login")
    , UrlParser.format LoginPage (s "teams" </> UrlParser.string </> s "login")
    ]
