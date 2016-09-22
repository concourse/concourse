module Layout exposing (Model, Msg, init, update, urlUpdate, view, subscriptions)

import Html exposing (Html)
import Html.Attributes as Attributes exposing (class, id)
import Html.App
import TopBar
import SideBar
import Routes
import SubPage

type alias Model =
  { proto : SubPage.Page
  , subModel : SubPage.Model
  , topModel : TopBar.Model
  , sideModel : SideBar.Model
  }

type Msg
  = SubMsg SubPage.Msg
  | TopMsg TopBar.Msg
  | SideMsg SideBar.Msg

init : SubPage.Page -> Routes.Concourse -> (Model, Cmd (Msg))
init sub route =
  let
    (subModel, subCmd) =
      sub.init route

    (topModel, topCmd) =
      TopBar.init route

    (sideModel, sideCmd) =
      SideBar.init
  in
    ( { proto = sub
      , subModel = subModel
      , topModel = topModel
      , sideModel = sideModel
      }
    , Cmd.batch
        [ Cmd.map SubMsg subCmd
        , Cmd.map TopMsg topCmd
        , Cmd.map SideMsg sideCmd
        ]
    )

update : Msg -> Model -> (Model, Cmd (Msg))
update msg model =
  case msg of
    SubMsg m ->
      Debug.log("got sub message") <|
        let
          (subModel, subCmd) = model.proto.update m model.subModel
        in
          ({ model | subModel = subModel }, Cmd.map SubMsg subCmd)

    TopMsg m ->
      let
        (topModel, topCmd) = TopBar.update m model.topModel
      in
        ({ model | topModel = topModel }, Cmd.map TopMsg topCmd)

    SideMsg m ->
      let
        (sideModel, sideCmd) = SideBar.update m model.sideModel
      in
        ({ model | sideModel = sideModel }, Cmd.map SideMsg sideCmd)

urlUpdate : Routes.ConcourseRoute -> Model -> (Model, Cmd (Msg))
urlUpdate route model =
  -- let
  --   (subModel, subCmd) =
  --     model.sub.init route
  --   -- (topModel, topCmd) =
  --   --   TopBar.init route
  --   --
  --   -- (sideModel, sideCmd) =
  --   --   SideBar.init
  -- in
  --   ( { model | subModel = subModel }
  --   , Cmd.batch
  --       [ Cmd.map SubMsg subCmd
  --       ]
  --   )
  (model, Cmd.none)

view : Model -> Html (Msg)
view model =
  Html.div [ class "content-frame" ]
    [ Html.div [ id "top-bar-app" ]
        [ Html.App.map TopMsg (TopBar.view model.topModel) ]
    , Html.div [ class "bottom" ]
        [ Html.div [ id "pipelines-nav-app", class "sidebar js-sidebar test" ]
            [ Html.App.map SideMsg (SideBar.view model.sideModel) ]
        , Html.div [ id "content" ]
            [ Html.App.map SubMsg (model.proto.view model.subModel) ]
        ]
    ]

subscriptions : Model -> Sub (Msg)
subscriptions model =
  Sub.none
