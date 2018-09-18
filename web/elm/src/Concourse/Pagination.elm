module Concourse.Pagination exposing (Paginated, Pagination, Page, Direction(..), fetch, parseLinks, equal)

import Dict exposing (Dict)
import Http
import Json.Decode
import Regex exposing (Regex)
import String
import Task exposing (Task)
import Maybe.Extra


type alias Paginated a =
    { content : List a
    , pagination : Pagination
    }


type alias Pagination =
    { previousPage : Maybe Page
    , nextPage : Maybe Page
    }


type alias Page =
    { direction : Direction
    , limit : Int
    }


type Direction
    = Since Int
    | Until Int
    | From Int
    | To Int


previousRel : String
previousRel =
    "previous"


nextRel : String
nextRel =
    "next"


linkHeaderRegex : Regex
linkHeaderRegex =
    Regex.regex ("<([^>]+)>; rel=\"(" ++ previousRel ++ "|" ++ nextRel ++ ")\"")


equal : Page -> Page -> Bool
equal one two =
    case one.direction of
        Since p ->
            case two.direction of
                Since pp ->
                    p == pp

                _ ->
                    False

        Until q ->
            case two.direction of
                Until qq ->
                    q == qq

                _ ->
                    False

        From f ->
            case two.direction of
                From ff ->
                    f == ff

                _ ->
                    False

        To t ->
            case two.direction of
                To tt ->
                    t == tt

                _ ->
                    False


fetch : Json.Decode.Decoder a -> String -> Maybe Page -> Task Http.Error (Paginated a)
fetch decode url page =
    Http.toTask <|
        Http.request
            { method = "GET"
            , headers = []
            , url = addParams url page
            , body = Http.emptyBody
            , expect = Http.expectStringResponse (parsePagination decode)
            , timeout = Nothing
            , withCredentials = False
            }


parsePagination : Json.Decode.Decoder a -> Http.Response String -> Result String (Paginated a)
parsePagination decode response =
    let
        pagination =
            parseLinks response

        decoded =
            Json.Decode.decodeString (Json.Decode.list decode) response.body
    in
        case decoded of
            Err err ->
                Err err

            Ok content ->
                Ok { content = content, pagination = pagination }


parseLinks : Http.Response String -> Pagination
parseLinks response =
    case Dict.get "link" <| keysToLower response.headers of
        Nothing ->
            Pagination Nothing Nothing

        Just commaSeparatedCraziness ->
            let
                headers =
                    String.split ", " commaSeparatedCraziness

                parsed =
                    Dict.fromList <| List.filterMap parseLinkTuple headers
            in
                Pagination
                    (Dict.get previousRel parsed |> Maybe.andThen parseParams)
                    (Dict.get nextRel parsed |> Maybe.andThen parseParams)


keysToLower : Dict String a -> Dict String a
keysToLower =
    Dict.fromList << List.map fstToLower << Dict.toList


fstToLower : ( String, a ) -> ( String, a )
fstToLower ( x, y ) =
    ( String.toLower x, y )


parseLinkTuple : String -> Maybe ( String, String )
parseLinkTuple header =
    case Regex.find (Regex.AtMost 1) linkHeaderRegex header of
        [] ->
            Nothing

        { submatches } :: _ ->
            case submatches of
                (Just url) :: (Just rel) :: _ ->
                    Just ( rel, url )

                _ ->
                    Nothing


parseParams : String -> Maybe Page
parseParams =
    fromQuery << Tuple.second << extractQuery


extractQuery : String -> ( String, Dict String String )
extractQuery url =
    case String.split "?" url of
        baseURL :: query :: _ ->
            ( baseURL, parseQuery query )

        _ ->
            ( url, Dict.empty )


setQuery : String -> Dict String String -> String
setQuery baseURL query =
    let
        params =
            String.join "&" <|
                List.map (\( k, v ) -> k ++ "=" ++ v) (Dict.toList query)
    in
        if params == "" then
            baseURL
        else
            baseURL ++ "?" ++ params


parseQuery : String -> Dict String String
parseQuery query =
    let
        parseParam p =
            case String.split "=" p of
                k :: vs ->
                    ( k, String.join "=" vs )

                [] ->
                    ( "", "" )
    in
        Dict.fromList <|
            List.map parseParam <|
                String.split "&" query


addParams : String -> Maybe Page -> String
addParams url page =
    let
        ( baseURL, query ) =
            extractQuery url
    in
        setQuery baseURL (Dict.union query (toQuery page))


fromQuery : Dict String String -> Maybe Page
fromQuery query =
    let
        limit =
            Maybe.withDefault 0 <|
                (Dict.get "limit" query |> Maybe.andThen parseNum)

        until =
            Maybe.map Until <|
                (Dict.get "until" query |> Maybe.andThen parseNum)

        since =
            Maybe.map Since <|
                (Dict.get "since" query |> Maybe.andThen parseNum)

        from =
            Maybe.map Since <|
                (Dict.get "from" query |> Maybe.andThen parseNum)

        to =
            Maybe.map Since <|
                (Dict.get "to" query |> Maybe.andThen parseNum)
    in
        Maybe.map (\direction -> { direction = direction, limit = limit }) <|
            Maybe.Extra.or until <|
                Maybe.Extra.or since <|
                    Maybe.Extra.or from to


toQuery : Maybe Page -> Dict String String
toQuery page =
    case page of
        Nothing ->
            Dict.empty

        Just somePage ->
            let
                directionParam =
                    case somePage.direction of
                        Since id ->
                            ( "since", toString id )

                        Until id ->
                            ( "until", toString id )

                        From id ->
                            ( "from", toString id )

                        To id ->
                            ( "to", toString id )

                limitParam =
                    ( "limit", toString somePage.limit )
            in
                Dict.fromList [ directionParam, limitParam ]


parseNum : String -> Maybe Int
parseNum =
    Result.toMaybe << String.toInt
