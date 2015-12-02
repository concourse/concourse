module Pagination where

import Dict
import Debug
import Http
import Regex exposing (Regex)
import String

type alias Pagination =
  { previousPage : Maybe String
  , nextPage : Maybe String
  }

previousRel : String
previousRel = "previous"

nextRel : String
nextRel = "next"

linkHeaderRegex : Regex
linkHeaderRegex =
  Regex.regex ("<([^>]+)>; rel=\"(" ++ previousRel ++ "|" ++ nextRel ++ ")\"")

parse : Http.Response -> Pagination
parse response =
  case Dict.get "Link" response.headers of
    Nothing ->
      Pagination Nothing Nothing

    Just commaSeparatedCraziness ->
      let
        headers = String.split ", " commaSeparatedCraziness
        parsed = Dict.fromList <| List.filterMap parseLinkTuple headers
      in
        Pagination (Dict.get previousRel parsed) (Dict.get nextRel parsed)

parseLinkTuple : String -> Maybe (String, String)
parseLinkTuple header =
  case Regex.find (Regex.AtMost 1) linkHeaderRegex header of
    [] ->
      Nothing

    {submatches} :: _ ->
      case submatches of
        (Just url :: Just rel :: _) ->
          Just (rel, url)

        _ ->
          Nothing
