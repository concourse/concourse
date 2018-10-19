module NewTopBar
    exposing
        ( Model
        , query
        , autocompleteOptions
        , viewConcourseLogo
        , queryStringFromSearch
        )

import Concourse
import Html.Styled as Html exposing (Html)
import Html.Styled.Attributes as HA
    exposing
        ( css
        , href
        , id
        , placeholder
        , src
        , type_
        , value
        )
import NewTopBar.Styles as Styles
import QueryString
import RemoteData exposing (RemoteData)
import SearchBar exposing (SearchBar(..))
import UserState exposing (UserState(..))


type alias Model r =
    { r
        | userState : UserState
        , userMenuVisible : Bool
        , searchBar : SearchBar
    }


query : Model r -> String
query model =
    case model.searchBar of
        Expanded r ->
            r.query

        _ ->
            ""


queryStringFromSearch : String -> String
queryStringFromSearch query =
    case query of
        "" ->
            QueryString.render QueryString.empty

        query ->
            QueryString.render <|
                QueryString.add "search" query QueryString.empty


viewConcourseLogo : List (Html msg)
viewConcourseLogo =
    [ Html.a
        [ css Styles.concourseLogo, href "#" ]
        []
    ]


autocompleteOptions : { a | query : String, teams : List Concourse.Team } -> List String
autocompleteOptions { query, teams } =
    case String.trim query of
        "" ->
            [ "status: ", "team: " ]

        "status:" ->
            [ "status: paused", "status: pending", "status: failed", "status: errored", "status: aborted", "status: running", "status: succeeded" ]

        "team:" ->
            List.map (\team -> "team: " ++ team.name) <| List.take 10 teams

        _ ->
            []
